package main

import (
	"fmt"
	"net/http"
	"path/filepath"
	"strconv"
	"strings"
	"text/template"
)

func getJobTitle(JobCode int) string {
	if JobCode > 0 {
		var JobTitle string
		rows, err := Phonebook.db.Query("select title from jobtitles where jobcode=?", JobCode)
		errcheck(err)
		defer rows.Close()
		for rows.Next() {
			errcheck(rows.Scan(&JobTitle))
			return JobTitle
		}
		errcheck(rows.Err())
	}
	return "Unknown"
}

func getNameFromUID(uid int) string {
	var FirstName string
	var LastName string
	var name string
	rows, err := Phonebook.db.Query("select firstname,lastname from people where uid=?", uid)
	errcheck(err)
	defer rows.Close()
	for rows.Next() {
		errcheck(rows.Scan(&FirstName, &LastName))
		name = fmt.Sprintf("%s %s", FirstName, LastName)
	}
	errcheck(rows.Err())
	return name
}

func getDepartmentFromDeptCode(deptcode int) string {
	var name string
	rows, err := Phonebook.db.Query("select name from departments where deptcode=?", deptcode)
	errcheck(err)
	defer rows.Close()
	for rows.Next() {
		errcheck(rows.Scan(&name))
	}
	errcheck(rows.Err())
	return name
}

func getReports(uid int, d *personDetail) {
	s := fmt.Sprintf("select uid,lastname,firstname,jobcode,primaryemail,officephone,cellphone from people where mgruid=%d AND status>0 order by lastname, firstname", uid)
	rows, err := Phonebook.db.Query(s)
	errcheck(err)
	defer rows.Close()
	for rows.Next() {
		var m person
		errcheck(rows.Scan(&m.UID, &m.LastName, &m.FirstName, &m.JobCode, &m.PrimaryEmail, &m.OfficePhone, &m.CellPhone))
		d.Reports = append(d.Reports, m)
	}
	errcheck(rows.Err())
}

func getImageFilename(uid int) string {
	pat := fmt.Sprintf("pictures/%d.*", uid)
	matches, err := filepath.Glob(pat)
	if err != nil {
		fmt.Printf("filepath.Glob(%s) returned error: %v\n", pat, err)
		return "/images/anon.png"
	}
	if len(matches) > 0 {
		return "/" + matches[0]
	}
	return "/images/anon.png"
}

func detailpopHandler(w http.ResponseWriter, r *http.Request) {
	var sess *session
	var ui uiSupport
	sess = nil
	if 0 < initHandlerSession(sess, &ui, w, r) {
		return
	}
	sess = ui.X
	breadcrumbBack(sess, 1)
	detailHandler(w, r)
}

func detailHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/html")
	var sess *session
	var ui uiSupport
	sess = nil
	if 0 < initHandlerSession(sess, &ui, w, r) {
		return
	}
	sess = ui.X

	var d personDetail
	d.Reports = make([]person, 0)

	var path string
	if strings.Contains(r.RequestURI, "pop") {
		path = "/detailpop/"
	} else {
		path = "/detail/"
	}
	uidstr := r.RequestURI[len(path):]
	uid := 0
	if len(uidstr) > 0 {
		uid, _ = strconv.Atoi(uidstr)
		d.UID = uid
	}
	breadcrumbAdd(sess, "Person", fmt.Sprintf("/detail/%d", uid))

	//=================================================================
	// SECURITY
	//=================================================================
	if !sess.elemPermsAny(ELEMPERSON, PERMVIEW|PERMOWNERVIEW) {
		ulog("ViewPersonDetail: Permission refusal on userid=%d (%s), role=%s\n", sess.UID, sess.Firstname, sess.Urole.Name)
		http.Redirect(w, r, "/search/", http.StatusFound)
		return
	}

	if uid > 0 {
		d.Image = getImageFilename(uid)
		rows, err := Phonebook.db.Query("select lastname,firstname,preferredname,jobcode,primaryemail,"+
			"officephone,cellphone,deptcode,cocode,mgruid,ClassCode,"+
			"HomeStreetAddress,HomeStreetAddress2,HomeCity,HomeState,HomePostalCode,HomeCountry "+
			"from people where uid=?", uid)
		errcheck(err)
		defer rows.Close()
		for rows.Next() {
			errcheck(rows.Scan(&d.LastName, &d.FirstName, &d.PreferredName, &d.JobCode, &d.PrimaryEmail,
				&d.OfficePhone, &d.CellPhone, &d.DeptCode, &d.CoCode, &d.MgrUID, &d.ClassCode,
				&d.HomeStreetAddress, &d.HomeStreetAddress2, &d.HomeCity,
				&d.HomeState, &d.HomePostalCode, &d.HomeCountry))
		}
		errcheck(rows.Err())
		d.MgrName = getNameFromUID(d.MgrUID)
		d.DeptName = getDepartmentFromDeptCode(d.DeptCode)
		d.JobTitle = getJobTitle(d.JobCode)
		getCompanyInfo(d.CoCode, &d.Company)
		getReports(uid, &d)
		d.Class = ui.ClassCodeToName[d.ClassCode]
	}
	t, _ := template.New("detail.html").Funcs(funcMap).ParseFiles("detail.html")
	ui.D = &d

	d.filterSecurityRead(sess, PERMVIEW)
	err := t.Execute(w, &ui)
	if nil != err {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}
