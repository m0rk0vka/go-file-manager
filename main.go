package main

import (
	"errors"
	"fmt"
	"hash/fnv"
	"html/template"
	"io"
	"log"
	"net/http"
	"os"
	"strconv"
)

type Page struct {
	Name     string
	Path     string
	IsFolder bool
	SubItems []Page
}

func NewPage() *Page {
	p := new(Page)
	p.Name = "My finder"
	p.Path = "/files/"
	p.IsFolder = true
	p.SubItems = []Page{
		{Name: "Loli", Path: "/files/Loli/", IsFolder: true, SubItems: []Page{{Name: "file.txt", Path: "/files/Loli/", IsFolder: false}}},
		{Name: "Holy", Path: "/files/Holy/", IsFolder: true},
		{Name: "file.txt", Path: "/files/", IsFolder: false},
	}

	return p
}

var (
	templates     = template.Must(template.ParseFiles(renderTemplateName("index")))
	dataPath      = "./data/"
	downloadPath  = "./download/"
	templatesPath = "./tmpl/"
	bonds         = map[string]uint32{}
	pages         = NewPage()
)

func hash(s string) uint32 {
	h := fnv.New32a()
	h.Write([]byte(s))
	ans := h.Sum32()
	// add bond of path and hash
	bonds[s] = ans
	return ans
}

func begin() error {
	// Clear data
	err := os.RemoveAll(dataPath)
	if err != nil {
		log.Fatal(err)
	}
	// Clear dowloads
	err = os.RemoveAll(downloadPath)
	if err != nil {
		log.Fatal(err)
	}

	// Create empty data dir
	if err := os.Mkdir(dataPath, os.ModePerm); err != nil {
		return err
	}

	// Create empty downloads dir
	if err := os.Mkdir(downloadPath, os.ModePerm); err != nil {
		return err
	}

	// Create demo files
	filePath := "/files/"
	filename := "file.txt"
	hashFilename := strconv.FormatUint(uint64(hash(filePath+filename)), 10)
	file, err := os.Create(dataPath + hashFilename)
	if err != nil {
		log.Fatal(err)
	}
	defer file.Close()

	filePath = "/files/Loli/"
	hashFilename = strconv.FormatUint(uint64(hash(filePath+filename)), 10)
	file1, err := os.Create(dataPath + hashFilename)
	if err != nil {
		log.Fatal(err)
	}
	defer file1.Close()

	log.Println("Successfully recreated demo")
	return nil
}

func renderFilename(filename string) string {
	return dataPath + filename
}

func renderTemplateName(tmpl string) string {
	return templatesPath + tmpl + ".html"
}

func renderTemplate(w http.ResponseWriter, tmpl string, p *Page) {
	err := templates.ExecuteTemplate(w, tmpl+".html", p)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
	log.Println("Successfully render template")
}

func showFolderHandler(w http.ResponseWriter, r *http.Request, page *Page) {
	renderTemplate(w, "index", page)
}

func redirect(w http.ResponseWriter, r *http.Request, p *Page) {
	//r.Method = http.MethodGet
	//r.URL.Path = p.Path
	http.Redirect(w, r, p.Path, http.StatusFound)
	log.Println("Successfully redirected")
}

func createFolderHandler(w http.ResponseWriter, r *http.Request) {
	fmt.Println("Inside create func")
	switch r.Method {
	case "POST":
		if err := r.ParseForm(); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		path := r.FormValue("path")
		page := getPageFromPath(path)
		if page == nil {
			http.NotFound(w, r)
			return
		}
		page.addNewFolder()
		redirect(w, r, page)
	default:
		renderTemplate(w, "index", pages)
	}
	log.Println("Successfully created folder")
}

func uploadFileHandler(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case "POST":
		uploadFile(w, r)
	default:
		renderTemplate(w, "index", pages)
	}
	log.Println("Successfully uploaded file")
}

func downloadFileHandler(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case "POST":
		if err := r.ParseForm(); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		path := r.FormValue("path")
		filename := r.FormValue("filename")
		page := getPageFromPath(path)
		filepath := path + filename

		// Create file
		possibleDownloadFilePath := downloadPath + filename
		// Cover file extension
		j := len(possibleDownloadFilePath) - 1
		for j >= 0 && possibleDownloadFilePath[j] != []byte(".")[0] {
			j -= 1
		}
		downloadFilePath := possibleDownloadFilePath
		_, err := os.Stat(downloadFilePath)
		i := 1
		for !errors.Is(err, os.ErrNotExist) {
			if j > 0 {
				downloadFilePath = possibleDownloadFilePath[:j] + "(" + strconv.Itoa(i) + ")" + possibleDownloadFilePath[j:]
			} else {
				downloadFilePath = possibleDownloadFilePath + "(" + strconv.Itoa(i) + ")"
			}
			_, err = os.Stat(downloadFilePath)
			i += 1
		}
		dst, err := os.Create(downloadFilePath)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		defer dst.Close()

		// Open file
		file, err := os.Open(dataPath + strconv.FormatUint(uint64(bonds[filepath]), 10))
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		defer file.Close()
		// Copy file
		if _, err := io.Copy(dst, file); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		// Redirect
		redirect(w, r, page)
	default:
		renderTemplate(w, "index", pages)
	}
	log.Println("Successfully downloaded file")
}

func deleteFileHandler(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case "POST":
		if err := r.ParseForm(); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		path := r.FormValue("path")
		filename := r.FormValue("filename")
		page := getPageFromPath(path)
		if page == nil {
			http.NotFound(w, r)
			return
		}
		page.deleteFile(filename)

		// Take hash
		filePath := path + filename
		fileDataPath := dataPath + strconv.FormatUint(uint64(bonds[filePath]), 10)
		// Delete from hash table
		delete(bonds, filePath)

		// Delete from data
		err := os.Remove(fileDataPath)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		redirect(w, r, page)
	default:
		renderTemplate(w, "index", pages)
	}
	log.Println("Successfully deleted file")
}

func changeFolderNameHandler(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case "POST":
		if err := r.ParseForm(); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		folderPath := r.FormValue("folderPath")
		newFolderName := r.FormValue("folderName")
		page := getPageFromPath(folderPath)
		page.changeName(newFolderName)
		rootPath := page.getRootPath()
		page = getPageFromPath(rootPath)
		redirect(w, r, page)
	}
	log.Println("Successfully changed folder name")
}

func changeFileNameHandler(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case "POST":
		if err := r.ParseForm(); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		filePath := r.FormValue("filePath")
		newFileName := r.FormValue("fileName")
		oldFileName := r.FormValue("oldFileName")
		page := getPageFromPath(filePath)
		err := page.changeFileName(newFileName, oldFileName)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		// Change name in bond
		oldFilePath := filePath + oldFileName
		filePath += newFileName
		oldHash := bonds[oldFilePath]
		bonds[filePath] = hash(filePath)

		// Change name in data
		newHashToString := strconv.FormatUint(uint64(bonds[filePath]), 10)
		oldHashToString := strconv.FormatUint(uint64(oldHash), 10)
		err = os.Rename(dataPath+oldHashToString, dataPath+newHashToString)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		redirect(w, r, page)
	}
	log.Println("Successfully changed file name")
}
func uploadFile(w http.ResponseWriter, r *http.Request) {
	r.ParseMultipartForm(10 << 20)

	// Get handler for filename, size and headers
	file, handler, err := r.FormFile("myFile")
	if err != nil {
		fmt.Println("Error Retrieving the File")
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	defer file.Close()

	if err := r.ParseForm(); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	//Add file to page struct
	path := r.FormValue("path")
	page := getPageFromPath(path)
	if page == nil {
		http.NotFound(w, r)
		return
	}
	page.addNewFile(handler.Filename)

	// Generate hash
	filepath := path + handler.Filename
	filename := strconv.FormatUint(uint64(hash(filepath)), 10)

	// Create file
	dst, err := os.Create(renderFilename(filename))
	defer dst.Close()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// Copy the uploaded file to the created file on the filesystem
	if _, err := io.Copy(dst, file); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	redirect(w, r, page)
}

func checkBeginPath(path string) bool {
	return path[:7] == "/files/"
}

func checkIfNameExist(p *Page, name string) bool {
	for _, item := range p.SubItems {
		if item.Name == name {
			return true
		}
	}

	return false
}

func findGoodName(p *Page) string {
	i := 0
	possibleName := "NewFolder"
	for checkIfNameExist(p, possibleName) {
		i += 1
		possibleName = possibleName[:9] + strconv.Itoa(i)
		fmt.Println(i)
	}
	return possibleName
}

func (p *Page) addNewFolder() {
	name := findGoodName(p)
	p.SubItems = append(p.SubItems, Page{Name: name, IsFolder: true, Path: p.Path + name + "/"})
}

func (p *Page) addNewFile(name string) {
	n := name
	i := 1
	if p.isFileExist(n) {
		n = n + " (" + strconv.Itoa(i) + ")"
	}
	for p.isFileExist(n) {
		i += 1
		j := len(n) - 1
		for j >= 0 && n[j] != []byte(" ")[0] {
			j -= 1
		}

		n = n[:j] + " (" + strconv.Itoa(i) + ")"
	}

	p.SubItems = append(p.SubItems, Page{Name: n, IsFolder: false, Path: p.Path})
}

func (p *Page) isFileExist(name string) bool {
	answer := false
	for _, item := range p.SubItems {
		if item.Name == name {
			answer = true
		}
	}

	return answer
}

func (p *Page) changeName(name string) {
	p.Name = name
	p.Path = p.getRootPath() + name + "/"
}

func (p *Page) changeFileName(new, old string) error {
	if p.isFileExist(new) {
		return fmt.Errorf("Duplicate file name %v\n", new)
	}

	index := -1
	for ind, item := range p.SubItems {
		if !item.IsFolder && item.Name == old {
			index = ind
		}
	}

	if index != -1 {
		p.SubItems[index].Name = new
	} else {
		return fmt.Errorf("Can't change file name %v to %v\n", old, new)
	}

	return nil
}

func (p *Page) getRootPath() string {
	i := len(p.Path) - 2
	for i >= 0 && p.Path[i] != []byte("/")[0] {
		i -= 1
	}
	return p.Path[:i+1]
}
func (p *Page) deleteFile(name string) {
	fileIndex := -1
	for ind, item := range p.SubItems {
		if item.Name == name && !item.IsFolder {
			fileIndex = ind
		}
	}

	if fileIndex != -1 {
		p.SubItems = append(p.SubItems[:fileIndex], p.SubItems[fileIndex+1:]...)
	}
}

func getPageFromPath(path string) *Page {
	if !checkBeginPath(path) {
		return nil
	}
	i := 7
	p := pages
	for i < len(path) {
		nextFolderName := []byte{}
		for i < len(path) && path[i] != []byte("/")[0] {
			nextFolderName = append(nextFolderName, path[i])
			i += 1
		}
		if i == len(path) {
			return nil
		}
		//check if folder exist
		isFolder := false
		folderIndex := -1
		for ind, item := range p.SubItems {
			if item.Name == string(nextFolderName) {
				isFolder = true
				folderIndex = ind
			}
		}
		// if folder does not exsit we return
		if !isFolder {
			return nil
		}

		//go to next folder
		p = &p.SubItems[folderIndex]

		// add 1, cause we need to move the slesh
		i += 1
	}
	return p
}

func makeHandler(fn func(http.ResponseWriter, *http.Request, *Page)) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		p := getPageFromPath(r.URL.Path)
		if p == nil {
			http.NotFound(w, r)
			return
		}
		fn(w, r, p)
	}
}

func main() {
	if err := begin(); err != nil {
		log.Fatal(err)
	}
	http.HandleFunc("/files/", makeHandler(showFolderHandler))
	http.HandleFunc("/createFolder", createFolderHandler)
	http.HandleFunc("/uploadFile", uploadFileHandler)
	http.HandleFunc("/deleteFile", deleteFileHandler)
	http.HandleFunc("/downloadFile", downloadFileHandler)
	http.HandleFunc("/changeFolderName", changeFolderNameHandler)
	http.HandleFunc("/changeFileName", changeFileNameHandler)
	log.Fatal(http.ListenAndServe(":8080", nil))
}
