package beego

import (
	"testing"
	"fmt"
	"regexp"
)

type TestPair struct {
	Route string
	Match string
}

var Xxxx = []TestPair{
	{`/admin/index/{<id(\d+)>}/{<name>}/{<age(\d{1,3})>}/{<code>}/{<oth([^.]+)>}`, `/admin/index/89/name_11111/77/code_111/bbb`},
	{`/admin/index/{<id(\d+)>}/{<name>}/{<age(\d{1,3})>}/{<code>}/{<oth([^.]+\.xml)>}`, `/admin/index/89/name_11111/77/code_111/bbb.xml`},
	{`/admin/index/{<id(\d+)>}/{<name>}/{<age(\d{1,3})>}/{<code>}/{<oth([^.]+\.xml)>}`, `/admin/index/89/name_11111/77/code_111/bbb/2234/mmm/.xml`},
}

func Test_ParseRoute(t *testing.T) {
	txt := "/admin/index/{<id(\\d+)>}/{<name>}/{<age(\\d{1,3})>}/{<code>}/{<oth([^.]+\\.xml)>}"
	reg, pattern, err := ParseRoute(txt)
	if err != nil {
		t.Fail()
	}
	t.Log(pattern)
	pa := "/admin/index/89/name_11111/77/code_111/bbb/rewrw/fsdfs.xml"
	if !reg.MatchString(pa) {
		t.Fatalf("no match")
	}
	vals, ok := NamedUrlValuesRegexpGroup(pa, reg)
	if !ok {
		t.Fatalf("can not match url")
	}

	if ok {
		if vals.Get("oth") != "bbb/rewrw/fsdfs.xml" {
			t.Fatalf("oth is : %v", vals.Get("oth"))
		}
	}

	for k, v := range Xxxx {
		regx, patt, err := ParseRoute(v.Route)
		if err != nil {
			t.Fail()
		}
		if !regx.MatchString(v.Match) {
			t.Errorf("%v \n %v \n %v \n %v", k, v.Route, v.Match, patt)
			continue
		}
	}
}

func Benchmark_ParamKeyValuePatternPair(b *testing.B) {

	param := "{<id(\\d+)>}"
	for i := 0; i < b.N; i++ {
		ParamKeyValuePatternPair(param)
	}
}

func Benchmark_ParseRoute(b *testing.B) {

	route := "/admin/index/{<id(\\d+)>}/{<name>}/{<a767(\\d{1,3})>}/{<code>}/ccc"
	for i := 0; i < b.N; i++ {
		ParseRoute(route)
	}
}

func Benchmark_NamedUrlValuesRegexpGroup(b *testing.B) {
	//测试而已
	reqPath := "/admin/index/9999/name/00/xxx/ccc"
	route := "/admin/index/{<id(\\d+)>}/{<name>}/{<a767(\\d{1,3})>}/{<code>}/ccc"
	reg, _, err := ParseRoute(route)
	if err != nil {
		b.Fail()
	}
	for i := 0; i < b.N; i++ {
		NamedUrlValuesRegexpGroup(reqPath, reg)
	}
}

func test_route(route, url string, printit bool) bool {
	reg, pattern, err := ParseRoute(route)
	if err != nil {
		if printit {
			fmt.Println(err)
		}
		return false
	}

	if reg != nil {
		values, matched := NamedUrlValuesRegexpGroup(url, reg)
		if matched {
			if printit {
				fmt.Println(values)
			}
			return true
		} else {
			if printit {
				fmt.Println(url, "not matched", reg.String())
			}
			return false
		}
	} else if *pattern == url {
		if printit {
			fmt.Println("static url :", url == *pattern)
		}
		return true
	} else {
		if printit {
			fmt.Println("reg and pattern all nil ?????")
		}
		return false
	}
}

var testData = []struct {
	Route     string
	UrlStr    string
	Result    bool
	Printable bool
}{
	{
		"/admin/index/{<id(\\d+)>}.{<ext((html|xml))>}",
		"/admin/index/888.html",
		true, false,
	},
	{
		"/admin/index/{<id(\\d+)>}/{<name>}/{<name>}/{<a6767(\\d{1,3})>}/{<code>}/ccc",
		"/admin/index/77/name1/name2/111/co778de/ccc",
		true, false,
	},
	{
		"/admin/index/ccc",
		"/admin/index/ccc",
		true, false,
	},
	{
		"/admin/index/{<id(\\d+)>}",
		"/admin/index/ccc",
		false, false,
	},
	{
		"/admin/index/{<id(\\d+)>}",
		"/admin/index/77/34",
		false, false,
	},
	{
		"/admin/index/{<id(\\d+)>}.html",
		"/admin/index/23.html",
		true, false,
	},
	{
		"/admin/index/{<id(\\d+)>}/{<name>}/{<name>}/{<a6767(\\d{1,3})>}/{<code>}/ccc",
		"/admin/index/77/name1/name2/111/co778de/",
		false, false,
	},
	{
		"/admin/index/{<id(\\d*)>}",
		"/admin/index/",
		true, false,
	},
	{
		"/admin/index/{<id(\\d+)>}.(html|xml)",
		"/admin/index/888.html",
		true, false,
	},
	{
		"/admin/中国/{<id(\\d+)>}.(html|xml)",
		"/admin/中国/888.html",
		true, false,
	},
	{
		"/admin/{<state>}/{<id(\\d+)>}.(html|xml)",
		"/admin/中国/888.html",
		true, false,
	},
	{
		"/admin/index/{<id(int)>}",
		"/admin/index/345678",
		true, false,
	},
	{
		"/admin/{<state>}/{<id(abc\\d+cde)>}.(html|xml)",
		"/admin/中国/abc888cde.html",
		true, false,
	},
	{
		"/parents(/{<pid((\\d+))>})+", //{<>}不能嵌套在正则表达式中,尽管可以通过测试
		"/parents/34/45",
		true, false,
	},
}

func Test_test1(t *testing.T) {
	for _, v := range testData {
		if test_route(v.Route, v.UrlStr, v.Printable) != v.Result {
			//fmt.Println(v.UrlStr ,"不匹配", v.Route)
			fmt.Println("测试失败:", v.UrlStr, v.Route)
			t.Fail()
		}
	}
}

func Benchmark_test1(b *testing.B) {
	route := "/admin/index/{<id(\\d+)>}/{<name(string)>}/{<name>}/{<a6767(\\d{1,3})>}/{<code>}/ccc"
	for i := 0; i < b.N; i++ {
		ParseRoute(route)
	}
}

func Benchmark_test2(b *testing.B) {
	route := "/admin/index/{<id(\\d+)>}/{<name(string)>}/{<name>}/{<a6767(\\d{1,3})>}/{<code>}/ccc"
	url := "/admin/index/77/name1/name2/111/co778de/ccc"
	for i := 0; i < b.N; i++ {
		test_route(route, url, false)
	}
}

func Test_reg(t *testing.T){
	//  /admin/editblog/2 ^/admin/editblog/(?P<id>[0-9]+)$
	reg,err:=regexp.Compile("^/admin/editblog/(?P<id>[0-9]+)$")
	if err!=nil {
		t.Fail()
	}

	b:=reg.MatchString("/admin/editblog/2")

	if !b {
		t.Fail()
	}
	vals,ok := NamedUrlValuesRegexpGroup("/admin/editblog/2", reg)

	if !ok {
		t.Fail()
	}

	fmt.Println(vals)
}
