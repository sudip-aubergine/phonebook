package main

import (
	"bytes"
	"compress/gzip"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"net/url"
	"os"
	"regexp"
	"strings"
	"time"
)

// Behavior describes a "behavior" of the virtual user.
// That is, a "habit" defining something the user does
type Behavior struct {
	Op     string // the operation performed
	Chance int    // number from 1 to 100 representing the percentage odds of doing this operation
	delay  int    // random number of seconds before executing the next command
}

// Profile is a named collection of Behaviors.  A profile describes
// how a virtual user will utilize the system.
type Profile struct {
	Name      string
	Behaviors []Behavior
}

// TestFailure provides a bit of detail about any test that fails...
// its name and table index as appropriate
type TestFailure struct {
	TestName string // name of test
	Context  string // some relevant context, the user, company, class, etc
	Reason   string // how was the failure noticed
	Index    int    // deprecated
}

// TestResults is a container for the number of passed and failed tests
type TestResults struct {
	SimUserID int           // the simulation uses this user id
	Pass      int           // number of tests that passed
	Fail      int           // number of tests that failed
	Failures  []TestFailure // more info about failures
}

// Tester profile does everything that Phonebook can do
var Tester Profile

// Regular Expressions for parsing replies
var reTitle = regexp.MustCompile("<title>")
var reTitleEnd = regexp.MustCompile("</title>")

func initProfiles() {
	Tester.Name = "Tester"
	Tester.Behaviors = []Behavior{
		{"search", 80, 5},
		{"detail", 10, 10},
		{"searchco", 2, 4},
		{"company", 1, 4},
		{"searchcl", 2, 10},
		{"class", 1, 5},
		{"weblogin", 2, 2},
		{"logoff", 2, 2},
	}
}

// logoff the supplied personDetail
//    returns true if login was successful
//            false if login failed
func logoff(d *personDetail) bool {
	URL := fmt.Sprintf("http://%s:%d/logoff/", App.Host, App.Port)
	hc := http.Client{}

	req, err := http.NewRequest("GET", URL, nil)
	errcheck(err)

	hdrs := []KeyVal{
		{"Host:", fmt.Sprintf("%s:%d", App.Host, App.Port)},
		{"Accept", "text/html,application/xhtml+xml,application/xml;q=0.9,image/webp,*/*;q=0.8"},
		{"Accept-Encoding", "gzip, deflate"},
		{"Accept-Language", "en-US,en;q=0.8"},
		{"Cache-Control", "max-age=0"},
		{"Connection", "keep-alive"},
		{"Content-Type", "application/x-www-form-urlencoded"},
		{"User-Agent", "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_10_5) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/47.0.2526.80 Safari/537.36"},
	}
	for i := 0; i < len(hdrs); i++ {
		req.Header.Add(hdrs[i].key, hdrs[i].value)
	}
	req.AddCookie(d.SessionCookie)
	resp, err := hc.Do(req)
	errcheck(err)
	defer resp.Body.Close()
	cookies := resp.Cookies()
	// fmt.Printf("Cookies:value: %+v\n", cookies)
	d.SessionCookie = nil
	for i := 0; i < len(cookies); i++ {
		if cookies[i].Name == "accord" {
			d.SessionCookie = cookies[i]
			break
		}
	}

	// Check that the server actually sent compressed data
	var reader io.ReadCloser
	switch resp.Header.Get("Content-Encoding") {
	case "gzip":
		fmt.Printf("gzip response\n")
		reader, err = gzip.NewReader(resp.Body)
		defer reader.Close()
	default:
		reader = resp.Body
	}

	// Verify that we were sent to the Sign In page...
	htmlData, err := ioutil.ReadAll(reader)
	errcheck(err)
	s := string(htmlData)
	m1 := reTitle.FindStringIndex(s)
	m2 := reTitleEnd.FindStringIndex(s)
	m := s[m1[1]:m2[0]]
	// fmt.Printf("Page returned = %s\n", m)
	if strings.Contains(m, "Accord") && strings.Contains(m, "Sign In") && d.SessionCookie == nil {
		// fmt.Printf("Logoff successful\n")
		return true
	}
	return false
}

// login the supplied personDetail
//    returns true if login was successful
//            false if login failed
func login(d *personDetail) bool {
	URL := fmt.Sprintf("http://%s:%d/weblogin/", App.Host, App.Port)
	hc := http.Client{}

	form := url.Values{}
	form.Add("username", d.UserName)
	form.Add("password", "accord")
	// req.PostForm = form
	// req.Header.Add("Content-Type", "application/x-www-form-urlencoded")

	req, err := http.NewRequest("POST", URL, bytes.NewBufferString(form.Encode()))
	errcheck(err)

	hdrs := []KeyVal{
		{"Host:", "localhost:8250"},
		{"Accept", "text/html,application/xhtml+xml,application/xml;q=0.9,image/webp,*/*;q=0.8"},
		{"Accept-Encoding", "gzip, deflate"},
		{"Accept-Language", "en-US,en;q=0.8"},
		{"Cache-Control", "max-age=0"},
		{"Connection", "keep-alive"},
		{"Content-Type", "application/x-www-form-urlencoded"},
		{"Origin", "http://localhost:8250"},
		{"Referer", "http://localhost:8250/signin/"},
		{"Upgrade-Insecure-Requests", "1"},
		{"User-Agent", "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_10_5) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/47.0.2526.80 Safari/537.36"},
	}
	for i := 0; i < len(hdrs); i++ {
		req.Header.Add(hdrs[i].key, hdrs[i].value)
	}
	// if 1 > 0 {
	// 	fmt.Printf("DumpRequest:\n")
	// 	dump, err := httputil.DumpRequest(req, false)
	// 	errcheck(err)
	// 	fmt.Printf("\n\ndumpRequest = %s\n", string(dump))
	// }

	resp, err := hc.Do(req)
	if nil != err {
		fmt.Printf("login:  hc.Do(req) returned error:  %#v\n", err)
		os.Exit(1)
	}
	defer resp.Body.Close()

	// if 1 > 0 {
	// 	fmt.Printf("DumpResponse:\n")
	// 	dump, err := httputil.DumpResponse(resp, true)
	// 	errcheck(err)
	// 	fmt.Printf("\n\ndumpResponse = %s\n", string(dump))
	// }

	// Verify if the response was ok
	if resp.StatusCode != http.StatusOK {
		fmt.Printf("Server return non-200 status: %v\n", resp.Status)
	}

	// dump headers...
	// fmt.Printf("Headers:\n")
	// for k, v := range resp.Header {
	// 	fmt.Println("key:", k, "value:", v)
	// }

	// cookies:
	cookies := resp.Cookies()
	// fmt.Printf("Cookies:value: %+v\n", cookies)
	for i := 0; i < len(cookies); i++ {
		if cookies[i].Name == "accord" {
			d.SessionCookie = cookies[i]
			break
		}
	}

	// Check that the server actually sent compressed data
	var reader io.ReadCloser
	switch resp.Header.Get("Content-Encoding") {
	case "gzip":
		fmt.Printf("gzip response\n")
		reader, err = gzip.NewReader(resp.Body)
		defer reader.Close()
	default:
		reader = resp.Body
	}

	// Verify that we were sent to the search page...
	htmlData, err := ioutil.ReadAll(reader)
	errcheck(err)
	s := string(htmlData)
	m1 := reTitle.FindStringIndex(s)
	m2 := reTitleEnd.FindStringIndex(s)
	m := s[m1[1]:m2[0]]
	// fmt.Printf("Page returned = %s\n", m)
	if strings.Contains(m, "Phonebook") && strings.Contains(m, "Search") && d.SessionCookie.Name == "accord" {
		// fmt.Printf("Login successful\n")
		return true
	}
	return false
}

// testResult consolidates a bunch of chores around running a test.
// returns true if the session cookie is nil
// otherwise returns false
func testResult(v *personDetail, testname string, success bool, tr *TestResults) bool {
	if success {
		tr.Pass++
	} else {
		tr.Fail++
	}
	if nil == v.SessionCookie && testname != "logoff" {
		fmt.Printf("usersim: could not find accord cookie after %s!\n", testname)
		return true
	}
	if nil != v.SessionCookie && testname == "logoff" {
		fmt.Printf("usersim: session cookie was not removed after %s!\n", testname)
		return true
	}
	return false
}

func usersimDoTest(v *personDetail, tr *TestResults) {
	// there should be no session in v now
	if testResult(v, "login", login(v), tr) {
		return
	}
	if testResult(v, "detail", viewPersonDetail(v, tr), tr) {
		return
	}
	if testResult(v, "adminView", adminViewTest(v, tr), tr) {
		return
	}
	if testResult(v, "adminEdit", adminEditTest(v, tr), tr) {
		return
	}
	if testResult(v, "saveAdminEdit", saveAdminEdit(v, tr), tr) {
		return
	}
	if testResult(v, "viewCompany", viewCompany(v, tr), tr) {
		return
	}
	if testResult(v, "adminEditCompany", adminEditCompany(v, tr), tr) {
		return
	}
	if testResult(v, "saveAdminEditCo", saveAdminEditCo(v, tr), tr) {
		return
	}
	if testResult(v, "viewClass", viewClass(v, tr), tr) {
		return
	}
	if testResult(v, "adminEditClass", adminEditClass(v, tr), tr) {
		return
	}
	if testResult(v, "saveAdminEditCo", saveAdminEditClass(v, tr), tr) {
		return
	}

	// after logoff, the session in v should be removed
	if testResult(v, "logoff", logoff(v), tr) {
		return
	}
}

func usersim(userindex, iterations int, finishTime time.Time, TestResChan chan TestResults, TestResChanAck chan int) {
	v := App.Peeps[userindex]
	tr := TestResults{v.UID, 0, 0, nil}

	if finishTime.Year() < 2015 {
		for i := 0; i < iterations; i++ {
			usersimDoTest(v, &tr)
		}
	} else {
		for time.Now().Before(finishTime) {
			usersimDoTest(v, &tr)
		}
	}

	TestResChan <- tr // push our results to the simulation executor
	<-TestResChanAck  // wait for receipt before continuing
}

func executeSimulation() {
	StartTime := time.Now()               // note the time we start
	TestResChan := make(chan TestResults) // usersim reports results via this struct
	TestResChanAck := make(chan int)      // ack receipt
	finishTime, _ := time.Parse(time.UnixDate, "Sat Mar  7 11:06:39 PST 2000")

	// fmt.Printf("Requested test duration: %v\n", App.TestDuration)

	if App.TestDuration.Seconds() > 0 {
		finishTime = time.Now().Add(App.TestDuration)
	}
	for j := 0; j < App.TestUsers; j++ {
		go usersim(j, App.TestIterations, finishTime, TestResChan, TestResChanAck)
		time.Sleep(500 * time.Millisecond)
	}

	var totTR TestResults                                                    // net results
	for i := App.FirstUserIndex; i < App.FirstUserIndex+App.TestUsers; i++ { // i is the number of usersims completed
		select {
		case tr := <-TestResChan: // get the data the usersim collected
			totTR.Fail += tr.Fail // update cumulative totals
			totTR.Pass += tr.Pass // update cumulative totals
			for j := 0; j < len(tr.Failures); j++ {
				totTR.Failures = append(totTR.Failures, tr.Failures[j])
			}
			TestResChanAck <- 1 // acknowledge receipt
		}
	}
	if len(totTR.Failures) > 0 {
		dumpTestErrors(&totTR)
	}
	Elapsed := time.Since(StartTime)
	fmt.Printf("Total Tests: %d   pass: %d   fail: %d\n", totTR.Fail+totTR.Pass, totTR.Pass, totTR.Fail)
	fmt.Printf("Random number seed for this run: %d\n", App.Seed)
	fmt.Printf("Test start time: %s\n", StartTime)
	fmt.Printf("Test end time  : %s\n", time.Now())
	fmt.Printf("Elapse time    : %s\n", Elapsed /*Round(Elapsed, 0.5e9)*/)
}
