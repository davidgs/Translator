package main

import (
	"bufio"
	"context"
	"fmt"
	"log"
	"os"
	"regexp"
	"strings"

	"cloud.google.com/go/translate"
	"google.golang.org/api/option"
	"golang.org/x/text/language"
)

func AuthTranslate(jsonPath, projectID string) (*translate.Client, context.Context, error){
	ctx := context.Background()
	client, err := translate.NewClient(ctx, option.WithCredentialsFile(jsonPath))
	if err != nil {
		log.Fatal(err)
		return client, ctx, err
	}
	return client, ctx, nil

}
// this is directly copy/pasted from Google example
func translateTextWithModel(targetLanguage, text, model string) (string, error) {

	lang, err := language.Parse(targetLanguage)
	if err != nil {
		return "", fmt.Errorf("language.Parse: %v", err)
	}
	client, ctx, err := AuthTranslate("micro-cacao-297616-fdd1edb1a227.json", "103373479946395174633")
	if err != nil {
		return "", fmt.Errorf("translate.NewClient: %v", err)
	}
	defer client.Close()
	resp, err := client.Translate(ctx, []string{text}, lang, &translate.Options{
		Model: model, // Either "nmt" or "base".
	})
	if err != nil {
		return "", fmt.Errorf("Translate: %v", err)
	}
	if len(resp) == 0 {
		return "", nil
	}
	return resp[0].Text, nil
}

// I get tired of typing this all the time
func checkError(err error) {
	if err != nil {
		log.Fatal(err)
	}
}

func xl(fromLang string, toLang string, xlate string) string {
	// fix URLs because google translate changes [link](http://you.link) to
	// [link] (http://your.link) and it *also* will translate any path
	// components, thus breaking your URLs.
	reg := regexp.MustCompile(`]\([-a-zA-Z0-9@:%._\+~#=\/]{1,256}\)`)
	// get all the URLs with a single RegEx, keep them for later.
	var foundUrls [][]byte = reg.FindAll([]byte(xlate), -1)
	translated, err := translateTextWithModel(toLang, xlate, "nmt")
	checkError(err)
	// a bunch of regexs to fix other broken stuff
	reg = regexp.MustCompile(` (\*\*) ([A-za-z0-9]+) (\*\*)`) // fix bolds (**foo**)
	translated = string(reg.ReplaceAll([]byte(translated), []byte(" $1$2$3")))
	reg = regexp.MustCompile(`&quot;`) // fix escaped quotes
	translated = string(reg.ReplaceAll([]byte(translated), []byte("\"")))
	reg = regexp.MustCompile(`&gt;`) //fix >
	translated = string(reg.ReplaceAll([]byte(translated), []byte(">")))
	reg = regexp.MustCompile(`&lt;`) // fix <
	translated = string(reg.ReplaceAll([]byte(translated), []byte("<")))
	reg = regexp.MustCompile(`&#39;`) // fix '
	translated = string(reg.ReplaceAll([]byte(translated), []byte("'")))
	reg = regexp.MustCompile(` (\*) ([A-za-z0-9]+) (\*)`) // fix underline (*foo*)
	translated = string(reg.ReplaceAll([]byte(translated), []byte("$1$2$3")))
	reg = regexp.MustCompile(`({{)(<)[ ]{1,3}([vV]ideo)`) // fix video shortcodes
	translated = string(reg.ReplaceAll([]byte(translated), []byte("$1$2 video")))
	reg = regexp.MustCompile(`({{)(<)[ ]{1,3}([yY]outube)`) // fix youtube shortcodes
	translated = string(reg.ReplaceAll([]byte(translated), []byte("$1$2 youtube")))
	// Now it's time to go back and replace all the fucked up urls ...
	reg = regexp.MustCompile(`] \([-a-zA-Z0-9@:%._\+~#=\/ ]{1,256}\)`)
	for x := 0; x < len(foundUrls); x++ {
		fmt.Println("FoundURL: ", string(foundUrls[x]))
		tmp := reg.FindIndex([]byte(translated))
		if tmp == nil {
			break
		}
		t := []byte(translated)
		translated = fmt.Sprintf("%s(%s%s", string(t[0:tmp[0]+1]), string(foundUrls[x][2:]), (string(t[tmp[1]:])))
	}
	return translated
}

// walk through the front matter, etc. and translate stuff
func doXlate(from string, lang string, readFile string, writeFile string) {
	file, err := os.Open(readFile)
	checkError(err)
	defer file.Close()
	xfile, err := os.Create(writeFile)
	checkError(err)
	defer xfile.Close()
	head := false
	code := false
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		ln := scanner.Text()
		if strings.HasPrefix(ln, "{{") {
			xfile.WriteString(ln + "\n")
			continue
		}
		if strings.HasPrefix(ln, "```") { // deal with in-line code
			xfile.WriteString(ln + "\n")
			code = !code
			continue
		}
		if code { // I don't translate code!
			xfile.WriteString(ln + "\n")
			continue
		}
		if string(ln) == "---" { // start and end of front matter
			xfile.WriteString(ln + "\n")
			head = !head
		} else if !head {
			if strings.HasPrefix(ln, "!") { // translate the ALT-TEXT not the image path
				bar := strings.Split(ln, "]")
				desc := strings.Split(bar[0], "[")
				translated := xl(from, lang, desc[1])
				xfile.WriteString("![" + translated + "]" + bar[1] + "\n")
			} else { // blank lines and everything else
				if ln == "" { // handle blank lines.
					xfile.WriteString("\n")
				} else { // everything else
					translated := xl(from, lang, ln)
					xfile.WriteString(translated + "\n")
				}
			}
		} else { // handle header fields
			headString := strings.Split(ln, ":")
			if headString[0] == "title" { // title
				translated := xl(from, lang, headString[1])
				xfile.WriteString(headString[0] + ": " + translated + "\n")
			} else if headString[0] == "description" { // description
				translated := xl(from, lang, headString[1])
				xfile.WriteString(headString[0] + ": " + translated + "\n")
			} else { // all other header fields left as-is
				xfile.WriteString(ln + "\n")
			}
		}
	}
	if err := scanner.Err(); err != nil {
		log.Fatal(err)
	}
	xfile.Close()
	file.Close()
}

// is a value in the array?
func isValueInList(value string, list []string) bool { // Test Written
	for _, v := range list {
		if v == value {
			return true
		}
	}
	return false
}

// future work for automagically translating all files.
func getFile(from string, path string, lang string) {
	thisDir, err := os.ReadDir(path)
	checkError(err)
	for _, f := range thisDir {
		if f.IsDir() {
			if f.Name() == "images" {
				continue
			}
			fmt.Println("going into ", path + "/" + f.Name())
			getFile(from, path+"/"+f.Name(), lang) // fucking hell, recursion!
		} else {
			if strings.Split(f.Name(), ".")[0] == "_index" || strings.Split(f.Name(), ".")[0] == "index" {
				fromFile := fmt.Sprintf("%s/%s.%s.md", path, strings.Split(f.Name(), ".")[0], from)
				toFile := fmt.Sprintf("%s/%s.%s.md", path, strings.Split(f.Name(), ".")[0], lang)
				// fmt.Println("From: ", fromFile)
				// fmt.Println(toFile)
				_, err := os.Stat(toFile)
				if !os.IsNotExist(err) {
					fmt.Printf("Already translated:\t %s/index.%s.md\n", path, lang)
					continue
				}
				// fmt.Printf("Found a file to translate:\t %s/%s\n", path, f.Name())
				fmt.Printf("Translating:\t %s\nto: \t\t%s\n", fromFile, toFile)
				doXlate(from, lang, fromFile, toFile)
				// }
				continue
			}
		}
	}
}

func main() {
	fromLang := "en"
	langs := [4]string{"nl", "fr", "de", "es"} // only doing these four languages right now
	dir := os.Args[1]                          // only doing a directory passed in
	for x := 0; x < len(langs); x++ {
		lang := langs[x]
		fmt.Print("Translating: \n" + dir + "\nTo: ")
		switch lang {
		case "es":
			fmt.Println("Spanish")
		case "fr":
			fmt.Println("French")
		case "de":
			fmt.Println("German")
		case "nl":
			fmt.Println("Dutch")
		}
		fi, err := os.Stat(dir)
		checkError(err)
		switch mode := fi.Mode(); {
		case mode.IsDir():
			// do directory stuff
			getFile(fromLang, dir, lang)
		case mode.IsRegular(): // we're just doing one file
			pt := strings.Split(dir, "/")
			fn := strings.Split(pt[len(pt)-1], ".")
			path := strings.TrimRight(dir, pt[len(pt)-1])
			writeFile := fmt.Sprintf("%s%s.%s.%s", path, fn[0], lang, fn[len(fn)-1])
			doXlate(fromLang, lang, dir, writeFile)
		}
	}

}
