package main

import (
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"text/template"
)

func delCheckError(caller string, sess *session, err error, s string, w http.ResponseWriter, r *http.Request) bool {
	if nil != err {
		ulog("%s: \"%s\"  err = %v\n", caller, s, err)
		fmt.Printf("%s: \"%s\"  err = %v\n", caller, s, err)
		http.Redirect(w, r, breadcrumbBack(sess, 2), http.StatusFound)
		return true
	}
	return false
}
func delPersonRefErrHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/html")
	var sess *session
	var ui uiSupport
	sess = nil
	if 0 < initHandlerSession(sess, &ui, w, r) {
		return
	}
	sess = ui.X

	path := "/delPersonRefErr/"
	uidstr := r.RequestURI[len(path):]
	if len(uidstr) > 0 {
		uid, _ := strconv.Atoi(uidstr)
		var pd personDetail
		if 0 != getPersonDetail(&pd, uid) {
			ulog("delPersonRefErrHandler: Error retrieving person information for userid=%d\n", uid)
			http.Redirect(w, r, "/search/", http.StatusFound)
			return
		}
		ui.D = &pd
		ui.D.filterSecurityRead(sess, PERMVIEW)
		breadcrumbAdd(sess, "Delete Person", fmt.Sprintf("/delPersonRefErr/%d", uid))

		s := fmt.Sprintf("select uid,lastname,firstname,preferredname,jobcode,primaryemail,officephone,cellphone,deptcode from people where mgruid=%d", uid)
		rows, err := Phonebook.db.Query(s)
		errcheck(err)
		defer rows.Close()
		var d searchResults

		for rows.Next() {
			var m person
			errcheck(rows.Scan(&m.UID, &m.LastName, &m.FirstName, &m.PreferredName, &m.JobCode, &m.PrimaryEmail, &m.OfficePhone, &m.CellPhone, &m.DeptCode))
			m.DeptName = getDepartmentFromDeptCode(m.DeptCode)
			pm := &m
			pm.filterSecurityRead(sess, PERMVIEW|PERMMOD)
			d.Matches = append(d.Matches, m)
		}
		errcheck(rows.Err())
		ui.R = &d
		t, _ := template.New("delPersonRefErr.html").Funcs(funcMap).ParseFiles("delPersonRefErr.html")
		err = t.Execute(w, &ui)
		if nil != err {
			http.Error(w, err.Error(), http.StatusInternalServerError)
		}
	} else {
		fmt.Fprintf(w, "uid = %s\nCould not convert to number\n", uidstr)
	}
}

func delPersonHandler(w http.ResponseWriter, r *http.Request) {
	var sess *session
	var ui uiSupport
	sess = nil
	if 0 < initHandlerSession(sess, &ui, w, r) {
		return
	}
	sess = ui.X

	// SECURITY
	if !hasAccess(sess, ELEMPERSON, "ElemEntity", PERMDEL) {
		ulog("Permissions refuse delPersonHandler page on userid=%d (%s), role=%s\n", sess.UID, sess.Firstname, sess.Urole.Name)
		http.Redirect(w, r, "/search/", http.StatusFound)
		return
	}

	// var d personDetail
	c := "delPersonHandler"
	m := strings.Split(r.RequestURI, "/")
	uidstr := m[len(m)-1]
	if len(uidstr) == 0 {
		fmt.Fprintf(w, "The RequestURI needs the person's uid. It was not found on the URI:  %s\n", r.RequestURI)
		return
	}
	uid, err := strconv.Atoi(uidstr)
	if err != nil {
		fmt.Fprintf(w, "Error converting uid to a number: %v. URI: %s\n", err, r.RequestURI)
		return
	}

	//===============================================================
	//  Check to see if this person manages anyone before deleting...
	//===============================================================
	s := fmt.Sprintf("select uid from people where MgrUID=%d", uid)
	rows, err := Phonebook.db.Query(s)
	errcheck(err)
	defer rows.Close()
	var refuid int
	count := 0
	for rows.Next() {
		errcheck(rows.Scan(&refuid))
		count++
	}
	errcheck(rows.Err())

	if count > 0 {
		http.Redirect(w, r, fmt.Sprintf("/delPersonRefErr/%d", uid), http.StatusFound)
		return
	}

	//===============================================================
	// in order to delete a person, we must delete all references
	// to the person in the following database tables:
	//		deductions
	//		compensation
	//===============================================================
	s = fmt.Sprintf("DELETE FROM people WHERE UID=%d", uid)
	stmt, err := Phonebook.db.Prepare(s)
	if delCheckError(c, sess, err, s, w, r) {
		return
	}
	_, err = stmt.Exec()
	if delCheckError(c, sess, err, s, w, r) {
		return
	}

	s = fmt.Sprintf("DELETE FROM deductions WHERE UID=%d", uid)
	stmt, err = Phonebook.db.Prepare(s)
	if delCheckError(c, sess, err, s, w, r) {
		return
	}
	_, err = stmt.Exec()
	if delCheckError(c, sess, err, s, w, r) {
		return
	}

	s = fmt.Sprintf("DELETE FROM compensation WHERE UID=%d", uid)
	stmt, err = Phonebook.db.Prepare(s)
	if delCheckError(c, sess, err, s, w, r) {
		return
	}
	_, err = stmt.Exec()
	if delCheckError(c, sess, err, s, w, r) {
		return
	}
	http.Redirect(w, r, "/search/", http.StatusFound)
}

func delClassRefErr(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/html")
	var sess *session
	var ui uiSupport
	sess = nil
	if 0 < initHandlerSession(sess, &ui, w, r) {
		return
	}
	sess = ui.X

	path := "/delClassRefErr/"
	costr := r.RequestURI[len(path):]
	if len(costr) > 0 {
		classcode, _ := strconv.Atoi(costr)
		var c class
		getClassInfo(classcode, &c)
		ui.A = &c
		ui.A.filterSecurityRead(sess, PERMVIEW)

		breadcrumbAdd(sess, "Delete Class", fmt.Sprintf("/delClassRefErr/%d", classcode))

		s := fmt.Sprintf("select uid,lastname,firstname,preferredname,jobcode,primaryemail,officephone,cellphone,deptcode from people where classcode=%d", classcode)
		rows, err := Phonebook.db.Query(s)
		errcheck(err)
		defer rows.Close()
		var d searchResults

		for rows.Next() {
			var m person
			errcheck(rows.Scan(&m.UID, &m.LastName, &m.FirstName, &m.PreferredName, &m.JobCode, &m.PrimaryEmail, &m.OfficePhone, &m.CellPhone, &m.DeptCode))
			m.DeptName = getDepartmentFromDeptCode(m.DeptCode)
			pm := &m
			pm.filterSecurityRead(sess, PERMVIEW|PERMMOD)
			d.Matches = append(d.Matches, m)
		}
		errcheck(rows.Err())
		ui.R = &d
		t, _ := template.New("delClassRefErr.html").Funcs(funcMap).ParseFiles("delClassRefErr.html")
		err = t.Execute(w, &ui)
		if nil != err {
			http.Error(w, err.Error(), http.StatusInternalServerError)
		}
	} else {
		fmt.Fprintf(w, "classcode = %s\nCould not convert to number\n", costr)
	}
}

func delClassHandler(w http.ResponseWriter, r *http.Request) {
	var sess *session
	var ui uiSupport
	sess = nil
	if 0 < initHandlerSession(sess, &ui, w, r) {
		return
	}
	sess = ui.X

	// SECURITY
	if !hasAccess(sess, ELEMCLASS, "ElemEntity", PERMDEL) {
		ulog("Permissions refuse delCoHandler page on userid=%d (%s), role=%s\n", sess.UID, sess.Firstname, sess.Urole.Name)
		http.Redirect(w, r, "/search/", http.StatusFound)
		return
	}

	c := "delClassHandler"
	m := strings.Split(r.RequestURI, "/")
	uidstr := m[len(m)-1]
	if len(uidstr) == 0 {
		fmt.Fprintf(w, "The RequestURI needs the class code. It was not found on the URI:  %s\n", r.RequestURI)
		return
	}
	ClassCode, err := strconv.Atoi(uidstr)
	if err != nil {
		fmt.Fprintf(w, "Error converting class code to a number: %v. URI: %s\n", err, r.RequestURI)
		return
	}

	//===============================================================
	//  Check for references to this class before deleting
	//===============================================================
	s := fmt.Sprintf("select uid from people where classcode=%d", ClassCode)
	rows, err := Phonebook.db.Query(s)
	errcheck(err)
	defer rows.Close()
	count := 0
	for rows.Next() {
		var refuid int
		errcheck(rows.Scan(&refuid))
		count++
	}
	errcheck(rows.Err())

	if count > 0 {
		http.Redirect(w, r, fmt.Sprintf("/delClassRefErr/%d", ClassCode), http.StatusFound)
		return
	}

	//===============================================================
	// in order to delete a person, we must delete all references
	// to the person in the following database tables:
	//		deductions
	//		compensationc,
	//===============================================================
	s = fmt.Sprintf("DELETE FROM classes WHERE ClassCode=%d", ClassCode)
	stmt, err := Phonebook.db.Prepare(s)
	if delCheckError(c, sess, err, s, w, r) {
		return
	}
	_, err = stmt.Exec()
	if delCheckError(c, sess, err, s, w, r) {
		return
	}
	// we've deleted it, now we need to reload our class list...
	loadClasses()
	http.Redirect(w, r, "/searchcl/", http.StatusFound)
}

func delCoRefErr(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/html")
	var sess *session
	var ui uiSupport
	sess = nil
	if 0 < initHandlerSession(sess, &ui, w, r) {
		return
	}
	sess = ui.X

	path := "/delCoRefErr/"
	costr := r.RequestURI[len(path):]
	if len(costr) > 0 {
		cocode, _ := strconv.Atoi(costr)
		var c company
		getCompanyInfo(cocode, &c)
		ui.C = &c
		ui.C.filterSecurityRead(sess, PERMVIEW)

		breadcrumbAdd(sess, "Delete Company", fmt.Sprintf("/delCoRefErr/%d", cocode))

		s := fmt.Sprintf("select uid,lastname,firstname,preferredname,jobcode,primaryemail,officephone,cellphone,deptcode from people where cocode=%d", cocode)
		rows, err := Phonebook.db.Query(s)
		errcheck(err)
		defer rows.Close()
		var d searchResults

		for rows.Next() {
			var m person
			errcheck(rows.Scan(&m.UID, &m.LastName, &m.FirstName, &m.PreferredName, &m.JobCode, &m.PrimaryEmail, &m.OfficePhone, &m.CellPhone, &m.DeptCode))
			m.DeptName = getDepartmentFromDeptCode(m.DeptCode)
			pm := &m
			pm.filterSecurityRead(sess, PERMVIEW|PERMMOD)
			d.Matches = append(d.Matches, m)
		}
		errcheck(rows.Err())
		ui.R = &d
		t, _ := template.New("delCoRefErr.html").Funcs(funcMap).ParseFiles("delCoRefErr.html")
		err = t.Execute(w, &ui)
		if nil != err {
			http.Error(w, err.Error(), http.StatusInternalServerError)
		}
	} else {
		fmt.Fprintf(w, "cocode = %s\nCould not convert to number\n", costr)
	}
}

func delCoHandler(w http.ResponseWriter, r *http.Request) {
	var sess *session
	var ui uiSupport
	sess = nil
	if 0 < initHandlerSession(sess, &ui, w, r) {
		return
	}
	sess = ui.X

	// SECURITY
	if !hasAccess(sess, ELEMCOMPANY, "ElemEntity", PERMDEL) {
		ulog("Permissions refuse delCoHandler page on userid=%d (%s), role=%s\n", sess.UID, sess.Firstname, sess.Urole.Name)
		http.Redirect(w, r, "/search/", http.StatusFound)
		return
	}

	c := "delCoHandler"
	m := strings.Split(r.RequestURI, "/")
	uidstr := m[len(m)-1]
	if len(uidstr) == 0 {
		fmt.Fprintf(w, "The RequestURI needs the company code. It was not found on the URI:  %s\n", r.RequestURI)
		return
	}
	CoCode, err := strconv.Atoi(uidstr)
	if err != nil {
		fmt.Fprintf(w, "Error converting company code to a number: %v. URI: %s\n", err, r.RequestURI)
		return
	}

	//===============================================================
	//  Check for references to this class before deleting
	//===============================================================
	s := fmt.Sprintf("select uid from people where CoCode=%d", CoCode)
	rows, err := Phonebook.db.Query(s)
	errcheck(err)
	defer rows.Close()
	count := 0
	for rows.Next() {
		var refuid int
		errcheck(rows.Scan(&refuid))
		count++
	}
	errcheck(rows.Err())

	if count > 0 {
		http.Redirect(w, r, fmt.Sprintf("/delCoRefErr/%d", CoCode), http.StatusFound)
		return
	}

	//===============================================================
	// in order to delete a person, we must delete all references
	// to the person in the following database tables:
	//		deductions
	//		compensation
	//===============================================================
	s = fmt.Sprintf("DELETE FROM companies WHERE CoCode=%d", CoCode)
	stmt, err := Phonebook.db.Prepare(s)
	if delCheckError(c, sess, err, s, w, r) {
		return
	}
	_, err = stmt.Exec()
	if delCheckError(c, sess, err, s, w, r) {
		return
	}
	// we've deleted it, now we need to reload our company list...
	loadCompanies()
	http.Redirect(w, r, "/searchco/", http.StatusFound)
}