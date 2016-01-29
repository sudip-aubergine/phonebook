package main

import (
	"crypto/sha512"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

func uploadFileCopy(from *multipart.File, toname string) error {
	to, err := os.Create(toname)
	if err != nil {
		ulog("uploadImageFile: Error on os.Create(%s) -- err=%v\n", toname, err)
		return err
	}
	defer to.Close()
	_, err = io.Copy(to, *from)
	if err != nil {
		ulog("savePersonDetailsHandler: Error writing picture file: %v\n", err)
	} else {
		ulog("File uploaded successfully to %s\n", toname)
	}
	return err
}

// uploadImageFile handles the uploading of a user's picture file and its
// placement in the pictures directory.
//
// Params
// usrfname - name of the file on the user's local system
// usrfile - the open file from the form return in the user's browser
// uid - uid of the user for whom the image applies
//
// Returns:  err from any os file function
func uploadImageFile(usrfname string, usrfile *multipart.File, uid int) error {
	// use the same filetype for the final filename
	ftype := filepath.Ext(usrfname)
	// ulog("user file type: %s\n", ftype)

	// the file name we'll use for this user's picture...
	picturefilename := fmt.Sprintf("pictures/%d%s", uid, ftype)
	// ulog("picturefilename: %s\n", picturefilename)

	//  delete old tmp file if exists:
	tmpFile := fmt.Sprintf("pictures/%d.tmp", uid)
	// ulog("tmpFile to delete if exists: %s\n", tmpFile)

	finfo, err := os.Stat(tmpFile)
	if os.IsNotExist(err) {
		ulog("%s was not found. Nothing to delete\n", tmpFile)
	} else {
		ulog("os.Stat(%s) returns:  err=%v,  finfo=%#v\n", tmpFile, err, finfo)
		err = os.Remove(tmpFile)
		ulog("os.Remove(%s) returns err=%v\n", tmpFile, err)
	}

	// copy the requested file to "<uid>.tmp"
	err = uploadFileCopy(usrfile, tmpFile)
	if nil != err {
		ulog("uploadFileCopy returned error: %v\n", err)
		return err
	}

	// see if there are any files that match the old filename MINUS the filetype...
	m, err := filepath.Glob(fmt.Sprintf("./pictures/%d.*", uid))
	if nil != err {
		ulog("filepath.Glob returned error: %v\n", err)
		return err
	}
	// ulog("filepath.Glob returned the following matches: %v\n", m)
	for i := 0; i < len(m); i++ {
		if filepath.Ext(m[i]) != ".tmp" {
			ulog("removing %s\n", m[i])
			err = os.Remove(m[i])
			if nil != err {
				ulog("error removing file: %s  err = %v\n", m[i], err)
				return err
			}
		}
	}

	// now move our newly uploaded picture into its proper name...
	err = os.Rename(tmpFile, picturefilename)
	if nil != err {
		ulog("os.Rename(%s,%s):  err = %v\n", tmpFile, picturefilename, err)
		return err
	}

	return nil
}

func savePersonDetailsHandler(w http.ResponseWriter, r *http.Request) {
	var sess *session
	var ui uiSupport
	sess = nil
	if 0 < initHandlerSession(sess, &ui, w, r) {
		return
	}
	sess = ui.X
	Phonebook.ReqCountersMem <- 1    // ask to access the shared mem, blocks until granted
	<-Phonebook.ReqCountersMemAck    // make sure we got it
	Counters.EditPerson++            // initialize our data
	Phonebook.ReqCountersMemAck <- 1 // tell Dispatcher we're done with the data

	var d personDetail
	path := "/savePersonDetails/"
	uidstr := r.RequestURI[len(path):]
	if len(uidstr) == 0 {
		fmt.Fprintf(w, "The RequestURI needs the person's uid. It was not found on the URI:  %s\n", r.RequestURI)
		return
	}
	uid, err := strconv.Atoi(uidstr)
	if err != nil {
		fmt.Fprintf(w, "Error converting uid to a number: %v. URI: %s\n", err, r.RequestURI)
		return
	}

	//=================================================================
	// SECURITY
	//=================================================================
	if !sess.elemPermsAny(ELEMPERSON, PERMOWNERMOD) {
		ulog("Permissions refuse savePersonDetails page on userid=%d (%s), role=%s\n", sess.UID, sess.Firstname, sess.Urole.Name)
		http.Redirect(w, r, "/search/", http.StatusFound)
		return
	}
	if uid != sess.UID {
		ulog("Permissions refuse savePersonDetails page on userid=%d (%s), role=%s trying to save for UID=%d\n", sess.UID, sess.Firstname, sess.Urole.Name, uid)
		http.Redirect(w, r, "/search/", http.StatusFound)
		return
	}
	d.UID = uid
	adminReadDetails(&d) //read current data
	action := strings.ToLower(r.FormValue("action"))
	if "save" == action {
		d.PreferredName = r.FormValue("PreferredName")
		d.PrimaryEmail = r.FormValue("PrimaryEmail")
		d.OfficePhone = r.FormValue("OfficePhone")
		d.CellPhone = r.FormValue("CellPhone")
		d.EmergencyContactPhone = r.FormValue("EmergencyContactPhone")
		d.EmergencyContactName = r.FormValue("EmergencyContactName")
		d.HomeStreetAddress = r.FormValue("HomeStreetAddress")
		d.HomeStreetAddress2 = r.FormValue("HomeStreetAddress2")
		d.HomeCity = r.FormValue("HomeCity")
		d.HomeState = r.FormValue("HomeState")
		d.HomePostalCode = r.FormValue("HomePostalCode")
		d.HomeCountry = r.FormValue("HomeCountry")

		if 0 == len(d.PreferredName) {
			sess.Firstname = d.FirstName
		} else {
			sess.Firstname = d.PreferredName
		}

		//=================================================================
		// handle image
		//=================================================================
		file, header, err := r.FormFile("picturefile")
		// fmt.Printf("file: %v, header: %v, err: %v\n", file, header, err)
		if nil == err {
			defer file.Close()
			err = uploadImageFile(header.Filename, &file, uid)
			if nil != err {
				ulog("uploadImageFile returned error: %v\n", err)
			}
			sess.ImageURL = getImageFilename(uid)
		} else {
			ulog("err loading picture: %v\n", err)
		}

		//=================================================================
		// Do the update
		//=================================================================
		_, err = Phonebook.prepstmt.updateMyDetails.Exec(d.PreferredName, d.PrimaryEmail, d.OfficePhone, d.CellPhone,
			d.EmergencyContactName, d.EmergencyContactPhone,
			d.HomeStreetAddress, d.HomeStreetAddress2, d.HomeCity, d.HomeState, d.HomePostalCode, d.HomeCountry, sess.UID,
			uid)
		if nil != err {
			http.Error(w, err.Error(), http.StatusInternalServerError)
		}

		password := r.FormValue("password")
		if "" != password {
			sha := sha512.Sum512([]byte(password))
			passhash := fmt.Sprintf("%x", sha)
			_, err = Phonebook.prepstmt.updatePasswd.Exec(passhash, uid)
			if nil != err {
				http.Error(w, err.Error(), http.StatusInternalServerError)
			}
		}
	}
	http.Redirect(w, r, breadcrumbBack(sess, 2), http.StatusFound)
}
