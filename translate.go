package main

import (
	"bytes"
	"fmt"
	"io"
	"io/fs"
	"bufio"
	"context"
	"log"
	"os"
	"strings"

	"cloud.google.com/go/translate"
	"golang.org/x/text/language"
)

// this is directly copy/pasted from Google example
func translateTextWithModel(targetLanguage, text, model string) (string, error) {
	// targetLanguage := "ja"
	// text := "The Go Gopher is cute"
	//model := "base"

	ctx := context.Background()

	lang, err := language.Parse(targetLanguage)
	if err != nil {
		return "", fmt.Errorf("language.Parse: %v", err)
	}

	client, err := translate.NewClient(ctx)
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


func checkError(err error) {
	if err != nil {
		log.Fatal(err)
	}
}

func xl(lang string, xlate string) string {
	// fix URLs because google translate changes [link](http://you.link) to
	// [link] (http://your.link) and it *also* will trnaslate any path
	// components, thus breaking your URLs.
	var url []string
	ind := bytes.Index([]byte(xlate), []byte("](")) // beginning of the url
	var tmp = []byte(xlate)
	for {
		if ind < 0 {
			break
		}
		n := tmp[ind+2:]
		end := bytes.Index([]byte(n), []byte(")")) // end of the url
		url = append(url, string(n[0:end]))
		tmp = n[end:]
		ind = bytes.Index([]byte(tmp), []byte("](")) // next url
	}
	translated, err := translateTextWithModel(lang, xlate, "base")
	checkError(err)
	translatedUnquote := strings.ReplaceAll(translated, "&quot;", "\"")
	translated = strings.ReplaceAll(translatedUnquote, "&#39;", "'")
	translatedUnquote = strings.ReplaceAll(translated, "&gt;", ">")
	translated = strings.ReplaceAll(translatedUnquote, "&lt;", ">")
	// translatedUnquote = strings.ReplaceAll(translated, "** ", "**")
	// translated = strings.ReplaceAll(translatedUnquote, " **", "**")

	// Now it's time to go back and replace all the fucked up urls ...
	final := ""
	if len(url) > 0 {
		ind = bytes.Index([]byte(translated), []byte("] ("))
		tmp = []byte(translated)
		uInd := 0
		for {
			if ind < 0 {
				break
			}
			start := ind + 2
			n := tmp[ind+2:]
			startString := string(tmp[0:start -1])
			end := bytes.Index(n, []byte(")"))
			final = final + startString + "(" + url[uInd]
			uInd++
			tmp = n[end:]
			mid := bytes.Index([]byte(tmp), []byte(" ["))
			if mid == -1 {
				final = final + string(tmp[:])
			} else {
				final = final + string(tmp[:mid])
			}
			ind = bytes.Index([]byte(tmp), []byte("] ("))
		}
	}
	if final == "" {
		return translated
	} else {
		return final
	}

}

// walk through the front matter, etc. and translate stuff
func doXlate(lang string, readFile string, writeFile string) {
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
				translated := xl(lang, desc[1])
				xfile.WriteString("![" + translated + "]" + bar[1] + "\n")

			}  else { // blank lines and everything else
				if ln == "" { // handle blank lines.
					xfile.WriteString("\n")
				} else { // everything else
					translated := xl(lang, ln)
					xfile.WriteString(translated + "\n")

				}
			}

		} else { // handle header fields
			headString := strings.Split(ln, ":")
			if headString[0] == "title" { // title
				translated := xl(lang, headString[1])
				xfile.WriteString(headString[0] + ": " + translated + "\n")
			} else if headString[0] == "description" { // description
				translated := xl(lang, headString[1])
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

func openFiles(readFile string, writeFile string) (io.Reader, os.File) {

	file, err := os.Open(readFile)
	checkError(err)
	defer file.Close()

	xfile, err := os.Create(writeFile)
	checkError(err)
	defer xfile.Close()

	return file, *xfile
}

func getFile(path string, thisDir []fs.DirEntry, lang string) {
	for _, f := range thisDir {
		if f.IsDir() {

			if f.Name() == "images" {
				continue
			}
			fmt.Print(path)
			fmt.Print("/")
			fmt.Println(f.Name())
			dirs, err := os.ReadDir(path + "/" + f.Name())
			checkError(err)

			getFile(path+"/"+f.Name(), dirs, lang)

		} else {
			if f.Name() == "_index"+"."+lang+"."+"md" || f.Name() == "index"+"."+lang+"."+"md" {
				continue
			}
			if f.Name() == "_index.en.md" {
				continue
				// checkFile := path + "/_index." + lang + ".md"
				// _, err := os.Stat(checkFile)
				// if os.IsNotExist(err) {
				// 	// readFile, writeFile := openFiles( )
				// 	doXlate(lang, path + "/" + f.Name(), path + "/_index." + lang + ".md")
				// }
			}
			if f.Name() == "index.en.md" {
				checkFile := path + "/index." + lang + ".md"
				_, err := os.Stat(checkFile)
				if os.IsNotExist(err) {
					//readFile, writeFile := openFiles(path + "/" + f.Name(), path + "/index." + lang + ".md")
					doXlate(lang, path+"/"+f.Name(), path+"/index."+lang+".md")
				}
			}
		}
	}
}
func main() {

	langs := [3]string{"fr", "de", "es"}
	dir := os.Args[1]
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
		}
		doXlate(lang, dir+"/index.en.md", dir+"/index."+lang+".md")
	}
	// foo := []byte("Así que una vez que estoy buscando otra oportunidad increíble en el espacio de la IO. Si usted ha leído a través de mi [sitio web] (https://davidgs.com) usted sabe que soy un pionero en la IO después de haber estado trabajando en la IO desde antes de que realmente era un IO. Eso sería [Proyecto Sun SPOT] (http://www.sunspotdev.org), que mató a Oracle o menos un año atrás. Estoy al menos seguir ayudando a que la comunidad viva un poco mediante la ejecución del nuevo sitio. Es curioso que después de todos estos años, todavía me contacté con regularidad por los usuarios del punto de Sun que todavía están utilizando la tecnología y que están en busca de apoyo.")
	// fmt.Println(hexdump.Dump(foo))
	//getFile(basePath, dirs, langs[x])
	//}

	//checkError(err)

	// fmt.Println("\n******************\n")
	// fmt.Println(TransBuff.String())
	// fmt.Println("\n******************\n")
	// foo, _ := ioutil.ReadFile("/Users/davidgs/github.com/DavidgsWeb/content/posts/pranks/door-prank/index.en.md")
	// str := string(foo)
	// fmt.Println(str)
	// translated, err := gtranslate.TranslateWithParams(
	// 	TransBuff.String(),
	// 	gtranslate.TranslationParams{
	// 		From: "en",
	// 		To:   "fr",
	// 	},
	// )
	// if err != nil {
	// 	panic(err)
	// }

	// en: Hello World | ja: こんにちは世界
}
