package web

import (
	"github.com/flosch/pongo2"
	"github.com/gorilla/csrf"
	"github.com/satori/go.uuid"
	"log"
	"net/http"
	"os"
	"strings"
	"time"
)

var staticCacheKey = strings.Replace(uuid.NewV4().String(), "-", "", -1)

func init() {
	err := pongo2.RegisterFilter(
		"isoformat",
		func(in *pongo2.Value, param *pongo2.Value) (*pongo2.Value, *pongo2.Error) {
			t, err := time.Parse(time.RFC3339, in.String())
			if err != nil {
				return nil, &pongo2.Error{OrigError: err}
			}

			return pongo2.AsValue(t.Format("January _2, 2006")), nil
		},
	)

	if err != nil {
		panic("failed to register template filter: " + err.Error())
	}
}

func pageTemplate(pagePath string) func() *pongo2.Template {
	var cached *pongo2.Template = nil
	return func() *pongo2.Template {
		if cached == nil {
			cached = pongo2.Must(pongo2.FromFile("templates/pages/" + pagePath))
		}

		return cached
	}
}

func partialTemplate(partialPath string) func() *pongo2.Template {
	var cached *pongo2.Template = nil
	return func() *pongo2.Template {
		if cached == nil {
			cached = pongo2.Must(pongo2.FromFile("templates/partials/" + partialPath))
		}

		return cached
	}
}

func renderHandler(pagePath string, context *pongo2.Context) http.HandlerFunc {
	t := pageTemplate(pagePath)()

	return func(w http.ResponseWriter, r *http.Request) {
		renderTemplate(w, r, t, context)
	}
}

func renderTemplate(w http.ResponseWriter, r *http.Request, template *pongo2.Template, context *pongo2.Context) {
	user := ctxGetUser(r)
	loggedIn := ctxGetLoggedIn(r)

	if os.Getenv("DEV_ENVIRONMENT") == "development" {
		staticCacheKey = strings.Replace(uuid.NewV4().String(), "-", "", -1)
	}

	newContext := pongo2.Context{
		"user":            user,
		"loggedIn":        loggedIn,
		"staticUrl":       os.Getenv("STATIC_URL"),
		"staticCacheKey":  staticCacheKey,
		"csrfToken":       csrf.Token(r),
		"csrfTokenHeader": "X-CSRF-Token",
		csrf.TemplateTag:  csrf.TemplateField(r),
	}

	if context != nil {
		newContext = newContext.Update(*context)
	}

	err := template.ExecuteWriter(newContext, w)
	if err != nil {
		log.Println("Failed to render template", err)
		http.Error(w, "Failed to render", http.StatusInternalServerError)
		return
	}
}