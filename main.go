//go:build ignore

package main

import (
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
}

func showFolderHandler(w http.ResponseWriter, r *http.Request, page *Page) {
	renderTemplate(w, "index", page)
}

func redirect(w http.ResponseWriter, r *http.Request, p *Page) {
	r.Method = http.MethodGet
	r.URL.Path = p.Path
	http.Redirect(w, r, p.Path, http.StatusCreated)

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
}

func uploadFileHandler(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case "POST":
		uploadFile(w, r)
	default:
		renderTemplate(w, "index", pages)
	}
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
		dst, err := os.Create(downloadPath + filename)
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
}

// TODO: delete from hash map and from data
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
		redirect(w, r, page)
	default:
		renderTemplate(w, "index", pages)
	}
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
}

// TODO change name in bond
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
		fmt.Println(newFileName, oldFileName, filePath)
		page := getPageFromPath(filePath)
		fmt.Println(page)
		page.changeFileName(newFileName, oldFileName)
		fmt.Println(page)
		redirect(w, r, page)
	}
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

	fmt.Printf("Uploaded File: %+v\n", handler.Filename)
	fmt.Printf("File Size: %+v\n", handler.Size)
	fmt.Printf("MIME Header: %+v\n", handler.Header)

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
	p.SubItems = append(p.SubItems, Page{Name: name, IsFolder: false, Path: p.Path})
}

func (p *Page) changeName(name string) {
	p.Name = name
	p.Path = p.getRootPath() + name + "/"
}

func (p *Page) changeFileName(new, old string) {
	fmt.Println("Items:")
	index := -1
	for ind, item := range p.SubItems {
		fmt.Println(item)
		if !item.IsFolder && item.Name == old {
			index = ind
		}
	}

	if index != -1 {
		p.SubItems[index].Name = new
	}
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
	fmt.Println(path)
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
	fmt.Println(p.Name, p.SubItems)
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
