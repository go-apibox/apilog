package apilog

import (
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"net"
	"net/http"
	"net/http/httputil"
	"regexp"
	"strings"
	"time"

	"github.com/go-apibox/api"
	"github.com/go-apibox/cache"
	"github.com/go-apibox/utils"
	"github.com/bitly/go-simplejson"
	"github.com/go-xorm/xorm"
)

type Log struct {
	app      *api.App
	disabled bool
	inited   bool

	trustedRproxyIpMap map[string]bool // 受信任的反向代理IP

	engine                   *xorm.Engine
	tableName                string
	detailTableName          string
	geoEnabled               bool
	geoCache                 *cache.Cache
	actionMatcher            *utils.Matcher
	codeMatcher              *utils.Matcher
	replaceRuleRegExps       []*regexp.Regexp
	replaceRuleRegReplaceTos []string
}

func NewLog(app *api.App) *Log {
	log := new(Log)
	log.app = app
	log.trustedRproxyIpMap = make(map[string]bool)

	cfg := app.Config
	disabled := cfg.GetDefaultBool("apilog.disabled", false)
	log.disabled = disabled
	if disabled {
		return log
	}

	log.init()
	return log
}

func (l *Log) init() {
	if l.inited {
		return
	}

	app := l.app
	cfg := app.Config
	dbType := cfg.GetDefaultString("apilog.db_type", "mysql")
	dbAlias := cfg.GetDefaultString("apilog.db_alias", "default")
	tableName := cfg.GetDefaultString("apilog.table.log", "api_log")
	detailTableName := cfg.GetDefaultString("apilog.table.detail", "api_log_detail")
	geoEnabled := cfg.GetDefaultBool("apilog.geo_enabled", true)
	actionWhitelist := cfg.GetDefaultStringArray("apilog.actions.whitelist", []string{"*"})
	actionBlacklist := cfg.GetDefaultStringArray("apilog.actions.blacklist", []string{})
	codeWhitelist := cfg.GetDefaultStringArray("apilog.codes.whitelist", []string{"*"})
	codeBlacklist := cfg.GetDefaultStringArray("apilog.codes.blacklist", []string{})
	replaceRules := cfg.GetDefaultMap("apilog.replace_rules", map[string]interface{}{})

	trustedIps := cfg.GetDefaultStringArray("trusted_rproxy_ips", []string{})
	for _, ip := range trustedIps {
		l.trustedRproxyIpMap[ip] = true
	}

	actionMatcher := utils.NewMatcher()
	actionMatcher.SetWhiteList(actionWhitelist)
	actionMatcher.SetBlackList(actionBlacklist)
	codeMatcher := utils.NewMatcher()
	codeMatcher.SetWhiteList(codeWhitelist)
	codeMatcher.SetBlackList(codeBlacklist)

	var engine *xorm.Engine
	var err error

	if tableName == "" {
		goto return_log
	}

	switch dbType {
	case "mysql":
		if api.HasDriver("mysql") {
			engine, err = app.DB.GetMysql(dbAlias)
			if err != nil {
				app.Logger.Warning("(apilog) can't get mysql with alias: %s, ignore apilog db config.", dbAlias)
				l.disabled = true
				goto return_log
			}
		} else {
			app.Logger.Warning("(apilog) mysql driver is not ready, ignore apilog db config.")
			l.disabled = true
			goto return_log
		}
	case "sqlite3":
		if api.HasDriver("sqlite3") {
			engine, err = app.DB.GetSqlite3(dbAlias)
			if err != nil {
				app.Logger.Warning("(apilog) can't get sqlite3 with alias: %s, ignore apilog db config.", dbAlias)
				l.disabled = true
				goto return_log
			}
		} else {
			app.Logger.Warning("(apilog) sqlite3 driver is not ready, ignore apilog db config.")
			l.disabled = true
			goto return_log
		}
	}

	// try to create table
	if tableName != "" {
		if dbType == "mysql" {
			engine.Exec(fmt.Sprintf(MYSQL_SCHEMA_LOG, tableName))
		} else if dbType == "sqlite3" {
			sql := strings.Replace(SQLITE3_SCHEMA_LOG, "%s", tableName, -1)
			sqls := strings.Split(sql, ";")
			for _, s := range sqls {
				engine.Exec(s)
			}
		}
	}
	if detailTableName != "" {
		if dbType == "mysql" {
			engine.Exec(fmt.Sprintf(MYSQL_SCHEMA_LOG_DETAIL, detailTableName))
		} else if dbType == "sqlite3" {
			sql := strings.Replace(SQLITE3_SCHEMA_LOG_DETAIL, "%s", detailTableName, -1)
			sqls := strings.Split(sql, ";")
			for _, s := range sqls {
				if s != "" {
					engine.Exec(s)
				}
			}
		}
	}

	// engine.ShowDebug = true

return_log:

	geoCache := cache.NewCache(5 * time.Minute) // 缓存5分钟

	l.engine = engine
	l.tableName = tableName
	l.detailTableName = detailTableName
	l.geoEnabled = geoEnabled
	l.geoCache = geoCache
	l.actionMatcher = actionMatcher
	l.codeMatcher = codeMatcher

	replaceRuleRegExps := make([]*regexp.Regexp, 0, len(replaceRules))
	replaceTos := make([]string, 0, len(replaceRules))
	for replaceRule, replaceTo := range replaceRules {
		regExp, err := regexp.Compile(replaceRule)
		if err != nil { // 忽略错误的表达式
			continue
		}
		replaceToStr, ok := replaceTo.(string)
		if !ok {
			continue
		}
		replaceRuleRegExps = append(replaceRuleRegExps, regExp)
		replaceTos = append(replaceTos, replaceToStr)
	}
	l.replaceRuleRegExps = replaceRuleRegExps
	l.replaceRuleRegReplaceTos = replaceTos

	// if err == nil {
	// 	app.AddRawIOHandler(l.rawIOHandler)
	// }
	l.inited = true
}

type nopCloser struct {
	io.Reader
}

func (nopCloser) Close() error { return nil }

func (l *Log) ServeHTTP(w http.ResponseWriter, r *http.Request, next http.HandlerFunc) {
	if l.disabled {
		next(w, r)
		return
	}

	if l.tableName == "" {
		next(w, r)
		return
	}

	startTime := time.Now()
	c, err := api.NewContext(l.app, w, r)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// check if action need be log
	action := c.Input.GetAction()
	if !l.actionMatcher.Match(action) {
		// next middleware
		next(w, r)
		return
	}

	// REF: http://stackoverflow.com/questions/29319783/go-logging-responses-to-incoming-http-requests-inside-http-handlefunc
	recorder := NewRecorder(w)

	// next middleware
	next(recorder, r)

	var code string
	returnData := c.Get("returnData")
	switch data := returnData.(type) {
	case *api.Error:
		code = data.Code
	default:
		code = "ok"
	}

	// check if action with this code need be log
	if !l.codeMatcher.Match(code) {
		return
	}

	duration := time.Since(startTime)
	elapseTime := duration.Nanoseconds()

	requestMethod := c.Input.Request.Method
	reqId := w.Header().Get("X-Request-Id")
	appId := c.Input.Get("api_appid")
	requestTime := startTime.Unix()

	var ip string
	remoteAddr := c.Input.Request.RemoteAddr
	if remoteAddr != "@" {
		ip, _, _ = net.SplitHostPort(remoteAddr)
	} else {
		// unix domain socket
		ip = "@"
	}

	// check X-Real-IP header
	isTrustIp := false
	if ip == "@" || utils.IsPrivateIp(ip) {
		isTrustIp = true
	} else if _, has := l.trustedRproxyIpMap[ip]; has {
		isTrustIp = true
	}
	if isTrustIp {
		realIp := c.Input.Request.Header.Get("X-Real-IP")
		if realIp != "" && realIp != "@" {
			ip = realIp
		}
	}

	rs, err := l.engine.Exec("INSERT INTO `"+l.tableName+"` "+
		"(request_id, request_method, app_id, action, code, ip, country, region, city, isp, request_time, elapse_time) "+
		"VALUES (?, ?, ?, ?, ?, ?, '', '', '', '', ?, ?)",
		reqId, requestMethod, appId, action, code, ip, requestTime, elapseTime,
	)
	if err != nil {
		l.app.Logger.Warning("(apilog) Insert log failed: %s", err.Error())
		return
	}
	logId, err := rs.LastInsertId()
	if err != nil {
		l.app.Logger.Warning("(apilog) Can not get log id: %s", err.Error())
		return
	}
	if l.detailTableName != "" {
		var reqStr, respStr string
		reqDump, err := httputil.DumpRequest(r, true)
		if err == nil {
			reqStr = string(reqDump)
		}

		resp := new(http.Response)
		resp.ProtoMajor = 1
		resp.ProtoMinor = 1
		resp.StatusCode = recorder.Code
		resp.Header = recorder.Header()
		resp.Body = nopCloser{recorder.Body}
		respDump, err := httputil.DumpResponse(resp, true)
		if err == nil {
			respStr = string(respDump)
		}

		if len(l.replaceRuleRegExps) > 0 {
			for i, re := range l.replaceRuleRegExps {
				reqStr = re.ReplaceAllString(reqStr, l.replaceRuleRegReplaceTos[i])
				respStr = re.ReplaceAllString(respStr, l.replaceRuleRegReplaceTos[i])
			}
		}
		l.engine.Exec("INSERT INTO `"+l.detailTableName+"` "+
			"(log_id, request_id, request, response) "+
			"VALUES (?, ?, ?, ?)",
			logId, reqId, reqStr, respStr,
		)
	}

	// insert log to table after handled
	go func() {
		country, region, city, isp := "", "", "", ""
		if l.geoEnabled && ip != "@" {
			country, region, city, isp, _ = l.getLocation(ip)
		}

		l.engine.Exec("UPDATE `"+l.tableName+"` "+
			"SET country=?, region=?, city=?, isp=? "+
			"WHERE log_id=?",
			country, region, city, isp, logId,
		)
	}()
}

func (l *Log) getLocation(ip string) (country, region, city, isp string, err error) {
	country, region, city, isp = "", "", "", ""

	if utils.IsPrivateIp(ip) {
		return "内网IP", "内网IP", "内网IP", "内网IP", nil
	}

	var body []byte
	bodyItem, exists := l.geoCache.Get(ip)
	if !exists {
		var resp *http.Response

		url := "http://ip.taobao.com/service/getIpInfo.php?ip=" + ip
		resp, err = http.Get(url)
		if err != nil {
			return
		}
		defer resp.Body.Close()

		body, err = ioutil.ReadAll(resp.Body)
		if err != nil {
			return
		}
		l.geoCache.Set(ip, body)
	} else {
		body, err = bodyItem.Bytes()
		if err != nil {
			return
		}
	}

	j, err := simplejson.NewJson(body)
	if err != nil {
		return
	}

	code, ok := j.CheckGet("code")
	if !ok {
		return
	}
	codeInt, err := code.Int()
	if err != nil || codeInt != 0 {
		err = errors.New("error code return in response")
		return
	}

	data, ok := j.CheckGet("data")
	if !ok {
		err = errors.New("missing data in response")
		return
	}

	country = data.Get("country").MustString()
	region = data.Get("region").MustString()
	city = data.Get("city").MustString()
	isp = data.Get("isp").MustString()
	return
}

// Enable enable the middle ware.
func (l *Log) Enable() {
	l.disabled = false
	l.init()
}

// Disable disable the middle ware.
func (l *Log) Disable() {
	l.disabled = true
}
