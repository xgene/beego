package beego

import (
	"fmt"
	"net/http"
	"net/url"
	"reflect"
	"regexp"
	"runtime"
	"strings"
)

type controllerInfo struct {
	pattern string
	regex   *regexp.Regexp
	controllerType reflect.Type
}

type userHandler struct {
	pattern string
	regex   *regexp.Regexp
	h http.Handler
}

type ControllerRegistor struct {
	routers []*controllerInfo
	filters      []http.HandlerFunc
	userHandlers map[string]*userHandler
}

func NewControllerRegistor() *ControllerRegistor {
	return &ControllerRegistor{routers: make([]*controllerInfo, 0), userHandlers: make(map[string]*userHandler)}
}

func (p *ControllerRegistor) Add(pattern string, c ControllerInterface) {

	regex, patt, err := ParseRoute(pattern)
	if err != nil {
		fmt.Println(err)
		return
	}

	//now create the Route
	t := reflect.Indirect(reflect.ValueOf(c)).Type()
	route := &controllerInfo{}
	route.regex = regex
	route.pattern = stringNilEmpty(patt)
	route.controllerType = t
	p.routers = append(p.routers, route)
}

func (p *ControllerRegistor) AddHandler(pattern string, c http.Handler) {

	regex, patt, err := ParseRoute(pattern)
	if err != nil {
		fmt.Println(err)
		return
	}
	//now create the Route
	uh := &userHandler{}
	uh.regex = regex
	uh.pattern = stringNilEmpty(patt)
	uh.h = c
	p.userHandlers[pattern] = uh
}

// Filter adds the middleware filter.
func (p *ControllerRegistor) Filter(filter http.HandlerFunc) {
	p.filters = append(p.filters, filter)
}

// FilterParam adds the middleware filter if the REST URL parameter exists.
func (p *ControllerRegistor) FilterParam(param string, filter http.HandlerFunc) {

	p.Filter(func(w http.ResponseWriter, r *http.Request) {
		p := r.URL.Query().Get(param)
		if len(p) > 0 {
			filter(w, r)
		}
	})
}

// FilterPrefixPath adds the middleware filter if the prefix path exists.
func (p *ControllerRegistor) FilterPrefixPath(path string, filter http.HandlerFunc) {
	p.Filter(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasPrefix(r.URL.Path, path) {
			filter(w, r)
		}
	})
}

// AutoRoute
func (p *ControllerRegistor) ServeHTTP(rw http.ResponseWriter, r *http.Request) {
	defer func() {
		if err := recover(); err != nil {
			errstr := fmt.Sprint(err)
			if handler, ok := ErrorMaps[errstr]; ok {
				handler(rw, r)
			} else {
				if !RecoverPanic {
					// go back to panic
					panic(err)
				} else {
					var stack string
					Critical("Handler crashed with error", err)
					for i := 1; ; i++ {
						_, file, line, ok := runtime.Caller(i)
						if !ok {
							break
						}
						Critical(file, line)
						if RunMode == "dev" {
							stack = stack + fmt.Sprintln(file, line)
						}
					}
					if RunMode == "dev" {
						ShowErr(err, rw, r, stack)
					}
				}
			}
		}
	}()
	w := &responseWriter{writer: rw}

	var runrouter *controllerInfo
	params := url.Values{}

	//static file server
	for prefix, staticDir := range StaticDir {
		if r.URL.Path == "/favicon.ico" {
			file := staticDir + r.URL.Path
			http.ServeFile(w, r, file)
			w.started = true
			return
		}
		if strings.HasPrefix(r.URL.Path, prefix) {
			file := staticDir + r.URL.Path[len(prefix):]
			http.ServeFile(w, r, file)
			w.started = true
			return
		}
	}

	requestPath := r.URL.Path
	r.ParseMultipartForm(MaxMemory)

	//user defined Handler
	for pattern, c := range p.userHandlers {
		if c.regex == nil && pattern == requestPath {
			c.h.ServeHTTP(rw, r)
			return
		} else if c.regex == nil {
			continue
		}

		if params, ok := NamedUrlValuesRegexpGroup(requestPath, c.regex); ok {
			if len(params) > 0 {
				values := r.URL.Query()
				for k, vals := range params {
					for _, v := range vals {
						values.Add(k, v)
						r.Form.Add(k, v)
					}
				}
				//println(r.URL.RawQuery)
				////reassemble query params and add to RawQuery
				//println(values.Encode())
				//println(r.URL.RawQuery)
				r.URL.RawQuery = url.Values(values).Encode() + "&" + r.URL.RawQuery
			}

			c.h.ServeHTTP(rw, r)
			return
		}
	}

	//first find path from the fixrouters to Improve Performance
	for _, route := range p.routers {
		if route.regex == nil {
			if len(requestPath) == len(route.pattern) && requestPath == route.pattern {
				runrouter = route
				break
			}
			continue
		}

		if params, ok := NamedUrlValuesRegexpGroup(requestPath, route.regex); ok {
			println(requestPath, route.regex.String(), len(params), route.controllerType.Name())
			runrouter = route
			if len(params) > 0 {
				values := r.URL.Query()
				for k, vals := range params {
					for _, v := range vals {
						values.Add(k, v)
						r.Form.Add(k, v)
					}
				}
				//println("r.URL.RawQuery=",r.URL.RawQuery)
				////reassemble query params and add to RawQuery
				//println("values.Encode()=",values.Encode())
				//println("r.URL.RawQuery",r.URL.RawQuery)
				//r.URL.RawQuery = params.Encode()
				r.URL.RawQuery = url.Values(values).Encode() + "&" + r.URL.RawQuery
				break
			}
		}
	}

	if runrouter != nil {
		//execute middleware filters
		for _, filter := range p.filters {
			filter(w, r)
			if w.started {
				return
			}
		}

		//Invoke the request handler
		vc := reflect.New(runrouter.controllerType)

		//call the controller init function
		init := vc.MethodByName("Init")
		in := make([]reflect.Value, 2)
		ct := &Context{ResponseWriter: w, Request: r, Params: params}
		in[0] = reflect.ValueOf(ct)
		in[1] = reflect.ValueOf(runrouter.controllerType.Name())
		init.Call(in)
		//call prepare function
		in = make([]reflect.Value, 0)
		method := vc.MethodByName("Prepare")
		method.Call(in)

		//if response has written,yes don't run next
		if !w.started {
			if r.Method == "GET" {
				method = vc.MethodByName("Get")
				method.Call(in)
			} else if r.Method == "HEAD" {
				method = vc.MethodByName("Head")
				method.Call(in)
			} else if r.Method == "DELETE" || (r.Method == "POST" && r.Form.Get("_method") == "delete") {
				method = vc.MethodByName("Delete")
				method.Call(in)
			} else if r.Method == "PUT" || (r.Method == "POST" && r.Form.Get("_method") == "put") {
				method = vc.MethodByName("Put")
				method.Call(in)
			} else if r.Method == "POST" {
				method = vc.MethodByName("Post")
				method.Call(in)
			} else if r.Method == "PATCH" {
				method = vc.MethodByName("Patch")
				method.Call(in)
			} else if r.Method == "OPTIONS" {
				method = vc.MethodByName("Options")
				method.Call(in)
			}
			if !w.started {
				if AutoRender {
					method = vc.MethodByName("Render")
					method.Call(in)
				}
				if !w.started {
					method = vc.MethodByName("Finish")
					method.Call(in)
				}
			}
		}
		method = vc.MethodByName("Destructor")
		method.Call(in)
	}

	//if no matches to url, throw a not found exception
	if w.started == false {
		if h, ok := ErrorMaps["404"]; ok {
			h(w, r)
		} else {
			http.NotFound(w, r)
		}
	}
}

//responseWriter is a wrapper for the http.ResponseWriter
//started set to true if response was written to then don't execute other handler
type responseWriter struct {
	writer  http.ResponseWriter
	started bool
	status  int
}

// Header returns the header map that will be sent by WriteHeader.
func (w *responseWriter) Header() http.Header {
	return w.writer.Header()
}

// Write writes the data to the connection as part of an HTTP reply,
// and sets `started` to true
func (w *responseWriter) Write(p []byte) (int, error) {
	w.started = true
	return w.writer.Write(p)
}

// WriteHeader sends an HTTP response header with status code,
// and sets `started` to true
func (w *responseWriter) WriteHeader(code int) {
	w.status = code
	w.started = true
	w.writer.WriteHeader(code)
}

const (
	PATTERN = `{<(?P<name>[A-Za-z_]+[A-Za-z_0-9]*)(\((?P<pattern>[^/<>]+)\))?>}`
	EMPTY   = "[^/]*"
)

//解析键值对失败产生恐慌
func replacement(s string) string {
	pair, matched := ParamKeyValuePatternPair(s)
	if matched {
		switch pair.Pattern {
		case "int":
			pair.Pattern = `\d+`
		case "string":
			pair.Pattern = `\w+`
		}
		//fmt.Println(s,"==>",pair.Name,"###",pair.Pattern)
		return `(?P<` + pair.Name + `>` + pair.Pattern + `)`
	}
	panic(fmt.Sprintf("片段'%s'无法匹配'%s'", s, PATTERN))
}

func ParseRoute(route string) (*regexp.Regexp, *string, error) {
	defer func() { //必须要先声明defer，否则不能捕获到panic异常
		if err := recover(); err != nil {
			fmt.Println(err) //这里的err其实就是panic传入的内容
		}
	}()
	re, err := regexp.Compile(PATTERN)
	if err != nil {
		return nil, nil, err
	}
	actual := re.ReplaceAllStringFunc(route, replacement) //这里可能产生panic
	if actual == route {
		return nil, &route, nil
	}
	reg, err := regexp.Compile("^" + actual + "$")
	if err != nil {
		return nil, nil, err
	}
	return reg, nil, nil
}

type ParamPair struct {
	Name    string
	Pattern string
}

// match regexp with string, and return a named group map
// Example: ParamKeyValuePatternPair
//   string: ":id(\\d+)"
//   return: ParamPair{ Name:"id", Pattern:"\\d+" }
func ParamKeyValuePatternPair(str string) (pair ParamPair, matched bool) {
	reg, _ := regexp.Compile(PATTERN)
	rst := reg.FindStringSubmatch(str)
	length := len(rst)
	if length < 3 {
		return
	}
	sn := reg.SubexpNames()
	if len(sn) < 3 {
		return
	}
	nameIdx, patternIdx := 0, 0
	for i := 0; i < len(sn) && i < length && nameIdx*patternIdx == 0; i++ {
		if sn[i] == "name" {
			nameIdx = i
		} else if sn[i] == "pattern" {
			patternIdx = i
		}
	}

	if nameIdx*patternIdx == 0 {
		return
	}

	pair = ParamPair{Name: rst[nameIdx], Pattern: rst[patternIdx]}
	if pair.Pattern == "" {
		pair.Pattern = EMPTY
	}
	matched = true
	return
}

// match regexp with string, and return a named group map
// Example:
//   regexp: "(?P<name>[A-Za-z]+)-(?P<age>\\d+)"
//   string: "CGC-30"
//   return: map[string][]string{ "name":["CGC"], "age":["30"] }
func NamedUrlValuesRegexpGroup(str string, reg *regexp.Regexp) (ng url.Values, matched bool) {
	rst := reg.FindStringSubmatch(str)
	if len(rst) < 1 {
		return
	}
	//for i,s :=range rst{
	//	fmt.Printf("%d => %s\n",i,s)
	//}
	ng = url.Values{}
	lenRst := len(rst)
	sn := reg.SubexpNames()
	for k, v := range sn {
		// SubexpNames contain the none named group,
		// so must filter v == ""
		//fmt.Printf("%s => %s\n",k,v)
		if k == 0 || v == "" {
			continue
		}
		if k+1 > lenRst {
			break
		}
		ng.Add(v, rst[k])
	}
	matched = true
	return
}

func stringNilEmpty(s *string) string {
	if s != nil {
		return *s
	}
	return ""
}
