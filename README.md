```
$ md2cfl upload --help
Upload converted markdown file to specified Confluence page.

Note that you may put Confluence-related parameters in front matter, e.g.:

---
confluence:
	base: "http://your.confluence.server"
	page: "583910399"
---

Usage:
  md2cfl upload [file] [flags]

Flags:
  -d, --dryrun         don't upload but print wiki text
  -h, --help           help for upload
  -P, --page string    page ID
  -t, --title string   page title (default "Page")

Global Flags:
  -b, --base string            Confluence base URL
  -p, --password string        Confluence password
  -s, --save-credential        Save username and password to system credential store
      --use-saved-credential   Use saved credential (default true)
  -u, --user string            Confluence user name
```