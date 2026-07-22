package admin

import (
	"embed"
	"html/template"
	"io/fs"
	"log"
	"net/http"

	"github.com/gin-gonic/gin"
)

//go:embed views/*
var viewsFS embed.FS

func SetupAdminPanel(r *gin.Engine) {
	subFS, err := fs.Sub(viewsFS, "views")
	if err != nil {
		log.Fatalf("Admin: Failed to create sub filesystem: %v", err)
	}

	tmpl := template.Must(template.ParseFS(viewsFS, "views/login.html"))

	r.GET("/admin/login", func(c *gin.Context) {
		tmpl.Execute(c.Writer, nil)
	})

	r.GET("/admin/*path", func(c *gin.Context) {
		if c.Param("path") == "/login" || c.Param("path") == "/login/" {
			tmpl.Execute(c.Writer, nil)
			return
		}
		c.FileFromFS("panel.html", http.FS(subFS))
	})

	r.GET("/admin", func(c *gin.Context) {
		c.FileFromFS("panel.html", http.FS(subFS))
	})

	staticFS := gin.WrapH(http.FileServer(http.FS(subFS)))
	r.GET("/admin/assets/*filepath", func(c *gin.Context) {
		c.Request.URL.Path = c.Param("filepath")
		staticFS(c)
	})
}
