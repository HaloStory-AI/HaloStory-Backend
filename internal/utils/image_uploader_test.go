package utils

import (
	"fmt"
	"log"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestImage_Uploader(t *testing.T) {
	log.Println("[TEST] Upload File")

	file, err := os.Open("./emotional.png")
	if err != nil {
		panic(err)
	}
	defer file.Close()
	
	fileInfo, err := file.Stat()
	if err != nil {
		panic(err)
	}

	log.Println(fmt.Sprintf("[TEST] Bytes Size: %d", fileInfo.Size()))
	
	url, err := UploadPicture(file)
	if err != nil {
		panic(err)
	}

	log.Println(url)

	assert.NotNil(t, url)
}