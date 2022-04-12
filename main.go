package main

import (
	"embed"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/skip2/go-qrcode"
	"github.com/zserge/lorca"
	"io/fs"
	"io/ioutil"
	"log"
	"net"
	"net/http"
	"os"
	"os/signal"
	"path"
	"path/filepath"
	"strings"
	"syscall"
)

//go:embed dist/dist*
var FS embed.FS

func main() {
	go func() {
		r := gin.Default()
		r.GET("/api/v1/addresses", AddressesController)
		r.GET("/uploads/:path", UploadsController)
		r.POST("/api/v1/texts", TextsController)
		r.GET("/api/v1/qrcodes", QrcodesController)
		// 让static/读取dist/dist文件夹里的文件
		staticFiles, _ := fs.Sub(FS, "dist/dist")
		r.StaticFS("/static", http.FS(staticFiles))
		//访问不存在的文件时，自动定向到打开index.html
		r.NoRoute(func(c *gin.Context) {
			path := c.Request.URL.Path
			if strings.HasPrefix(path, "/static/") {
				reader, err := staticFiles.Open("index.html")
				if err != nil {
					log.Fatal(err)
				}
				defer reader.Close()
				stat, err := reader.Stat()
				if err != nil {
					log.Fatal(err)
				}
				c.DataFromReader(http.StatusOK, stat.Size(), "text/html;charset=utf-8", reader, nil)
			} else {
				c.Status(http.StatusNotFound)
			}
		})
		r.Run(":8000")
	}()
	// Create UI with basic HTML passed via data URI
	ui, err := lorca.New("http://localhost:8000/static/index.html", "", 800, 600)
	if err != nil {
		log.Fatal(err)
	}
	ch := make(chan os.Signal, 1)
	signal.Notify(ch, syscall.SIGINT, syscall.SIGTERM)
	defer ui.Close()
	select {
	// Wait until UI window is closed
	case <-ui.Done():
	//系统中断
	case <-ch:
	}
}

func AddressesController(c *gin.Context) {
	addrs, _ := net.InterfaceAddrs()
	var result []string
	//遍历所有IP地址
	for _, address := range addrs {
		// check the address type and if it is not a loopback the display it
		if ipnet, ok := address.(*net.IPNet); ok && !ipnet.IP.IsLoopback() {
			if ipnet.IP.To4() != nil {
				result = append(result, ipnet.IP.String())
			}
		}
	}
	//将IP地址转为JSON再写入到HTTP相应
	c.JSON(http.StatusOK, gin.H{"addresses": result})
}
func GetUploadsDir() (uploads string) {
	exe, err := os.Executable()
	if err != nil {
		log.Fatal(err)
	}
	dir := filepath.Dir(exe)
	uploads = filepath.Join(dir, "uploads")
	return
}
func UploadsController(c *gin.Context) {
	if path := c.Param("path"); path != "" {
		target := filepath.Join(GetUploadsDir(), path)
		//
		c.Header("Content-Description", "File Transfer")
		c.Header("Content-Transfer-Encoding", "binary")
		c.Header("Content-Disposition", "attachment; filename="+path)
		c.Header("Content-Type", "application/octet-stream")
		c.File(target)
	} else {
		c.Status(http.StatusNotFound)
	}
}
func TextsController(c *gin.Context) {
	var json struct {
		Raw string `json:"raw"`
	}
	if err := c.ShouldBindJSON(&json); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
	} else {
		exe, err := os.Executable() // 获取当前执行文件的路径
		if err != nil {
			log.Fatal(err)
		}
		dir := filepath.Dir(exe) // 获取当前执行文件的目录
		if err != nil {
			log.Fatal(err)
		}
		filename := uuid.New().String()          // 生成一个文件名
		uploads := filepath.Join(dir, "uploads") // 拼接 uploads 的绝对路径
		err = os.MkdirAll(uploads, os.ModePerm)  // 创建 uploads 目录,设置目录权限777
		if err != nil {
			log.Fatal(err)
		}
		fullpath := path.Join("uploads", filename+".txt")                            // 拼接文件的绝对路径（不含 exe 所在目录）
		err = ioutil.WriteFile(filepath.Join(dir, fullpath), []byte(json.Raw), 0644) // 将 json.Raw 写入文件
		if err != nil {
			log.Fatal(err)
		}
		c.JSON(http.StatusOK, gin.H{"url": "/" + fullpath}) // 返回文件的绝对路径（不含 exe 所在目录）
	}
}

//二维码接口
func QrcodesController(c *gin.Context) {
	if content := c.Query("content"); content != "" {
		png, err := qrcode.Encode(content, qrcode.Medium, 256)
		if err != nil {
			log.Fatal(err)
		}
		c.Data(http.StatusOK, "image/png", png)
	} else {
		c.Status(http.StatusBadRequest)
	}
}
