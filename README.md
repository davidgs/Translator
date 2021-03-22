# Hugo Page Translator

This is a go program that will take your Hugo blog post entries and translate them to whatever languages you want.

You will need a Google account, with a project, and Google Trnaslate API enabled.

## Google setup

Start with the [Google Documentation](https://cloud.google.com/translate/docs/setup) to get your system setup for using the API. you will need to make sure that the api `json` file is stored locally, in the same directory as this program.

## Using this program

Once you have google translate set up and working, using this program consists of:

```
% go get
% go build translate.go
% ./translate <full path to the file to be translated.md>
```

Right now, it is hard-coded to translate whatever file you tell it to French, German and Spanish. I am working on making this more robust so you can list languages, list files, or just give it a directory to scan and find all the underlying `index.en.md` files in it.

**Note:** You should have all of your blog posts in `index.en.md` files, not just `index.md` files or this program won't find them.

## Caveats

This was written specifically for me, and my Hugo setup using the [Toha](https://toha-guides.netlify.app) theme. It may or may not work for your Hugo theme.

PRs etc. always welcomed!