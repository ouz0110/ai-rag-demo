package main

import (
	"bytes"
	"fmt"
	"io"
	"io/fs"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"sync"
)

const (
	ModuleName     = "ai-rag-demo"
	ConcurrencyNum = 8
)

var (
	CompileCommandOptions = []string{
		"--proto_path=./api",
		"--proto_path=./api/third_party",
		"--go_out=paths=source_relative:./api",
		"--go-http_out=paths=source_relative:./api",
		"--go-grpc_out=paths=source_relative:./api",
		"--go-errors_out=paths=source_relative:./api",
		"--validate_out=lang=go:./api",
	}
	// 异步编译列表
	pathList = []string{
		"api/common",
		"api/base/v1",
		"api/nocli/v1",
	}
	// 同步编译列表
	pathSyncList = []string{}

	// OpenApiDirs 指定需要生成 openapi.yaml 的 proto 目录列表。
	// 为空时不生成 openapi。会把这些目录的 proto 一起交给一次 protoc --openapi_out，
	// 输出到 OpenApiOutDir 目录，最终生成一个统一的 openapi.yaml。
	//
	// 示例：
	OpenApiDirs = []string{
		"api/base/v1",
		"api/nocli/v1",
	}
	// OpenApiOutDir openapi 输出目录，相对于项目根目录。
	OpenApiOutDir = "."
)

func protocApiCompile(apiPath string) {
	protoList := make([]string, 0)
	err := filepath.Walk(apiPath, func(path string, info fs.FileInfo, err error) error {
		if info.IsDir() {
			return nil
		}
		if strings.HasSuffix(path, ".proto") {
			protoList = append(protoList, path)
		}
		return nil
	})
	if err != nil {
		log.Fatalln("> protoc_api_compile failure:", err)
	}

	cmd := exec.Command("protoc", append(CompileCommandOptions, protoList...)...)
	commandStderrReader := bytes.NewBuffer(nil)
	cmd.Stderr = commandStderrReader

	err = cmd.Start()
	if err != nil {
		log.Fatalln("> protoc_api_compile failure:", err)
	}

	err = cmd.Wait()
	if err != nil {
		_, codeFile, codeLine, _ := runtime.Caller(0)
		log.Fatalln(
			"> protoc_api_compile failure:",
			codeFile+":"+strconv.FormatInt(int64(codeLine), 10)+":", err,
			"\n\ncommand:\n"+strings.Join(cmd.Args, " "),
			"\n\noutput:\n"+commandStderrReader.String(),
		)
	}
	fmt.Println("> protoc_api_compile:", apiPath, "done")
}

func protocOpenApiGenerate() {
	if len(OpenApiDirs) == 0 {
		return
	}

	protoList := make([]string, 0)
	for _, dir := range OpenApiDirs {
		err := filepath.Walk(dir, func(path string, info fs.FileInfo, err error) error {
			if err != nil {
				return err
			}
			if info.IsDir() {
				return nil
			}
			if strings.HasSuffix(path, ".proto") {
				protoList = append(protoList, path)
			}
			return nil
		})
		if err != nil {
			log.Fatalln("> protoc_openapi_generate failure:", err)
		}
	}

	if len(protoList) == 0 {
		fmt.Println("> protoc_openapi_generate: no proto files found in OpenApiDirs, skip")
		return
	}

	openapiOut := "--openapi_out=fq_schema_naming=true,default_response=false,enum_type=string:" + OpenApiOutDir
	openapiOptions := []string{
		"--proto_path=./api",
		"--proto_path=./api/third_party",
		openapiOut,
	}
	cmd := exec.Command("protoc", append(openapiOptions, protoList...)...)
	commandStderrReader := bytes.NewBuffer(nil)
	cmd.Stderr = commandStderrReader

	err := cmd.Start()
	if err != nil {
		log.Fatalln("> protoc_openapi_generate failure:", err)
	}

	err = cmd.Wait()
	if err != nil {
		_, codeFile, codeLine, _ := runtime.Caller(0)
		log.Fatalln(
			"> protoc_openapi_generate failure:",
			codeFile+":"+strconv.FormatInt(int64(codeLine), 10)+":", err,
			"\n\ncommand:\n"+strings.Join(cmd.Args, " "),
			"\n\noutput:\n"+commandStderrReader.String(),
		)
	}
	fmt.Println("> protoc_openapi_generate:", OpenApiOutDir, "done")
}

func main() {
	var wg sync.WaitGroup
	semaphoreCh := make(chan struct{}, ConcurrencyNum)
	log.SetFlags(log.Lmsgprefix)

	// Protobuf编译处理

	for i := range pathList {
		wg.Add(1)
		semaphoreCh <- struct{}{}
		go func(i int) {
			defer wg.Done()
			defer func() {
				<-semaphoreCh
			}()

			protocApiCompile(pathList[i])
		}(i)
	}
	wg.Wait()

	for i := range pathSyncList {
		protocApiCompile(pathSyncList[i])
	}

	fmt.Println("> protoc_api_compile completed")

	protocOpenApiGenerate()

	fmt.Println("> protoc_openapi_generate completed")

	// Protobuf编译产物引用路径处理

	protoDirFindList := make([]string, 0)
	protoDirReplaceList := make([]string, 0)
	protoGoList := make([]string, 0)
	err := filepath.Walk("api", func(path string, info fs.FileInfo, err error) error {
		if info.IsDir() {
			protoDirFindList = append(protoDirFindList, `"`+strings.TrimPrefix(path, "api/")+`"`)
			protoDirReplaceList = append(protoDirReplaceList, `"`+ModuleName+"/"+path+`"`)
		} else {
			if strings.HasSuffix(path, ".go") || strings.HasSuffix(path, ".go-e") {
				protoGoList = append(protoGoList, path)
			}
		}
		return nil
	})
	if err != nil {
		log.Fatalln("> protoc_api_import failure:", err)
	}

	for i := range protoGoList {
		wg.Add(1)
		semaphoreCh <- struct{}{}
		go func(i int) {
			defer wg.Done()
			defer func() {
				<-semaphoreCh
			}()

			protoGoFile, err := os.Open(protoGoList[i])
			if err != nil {
				log.Fatalln("> protoc_api_import failure:", err)
			}
			defer protoGoFile.Close()

			protoGoFileBytes, err := io.ReadAll(protoGoFile)
			if err != nil {
				log.Fatalln("> protoc_api_import failure:", err)
			}
			protoGoFileString := string(protoGoFileBytes)

			var replaceFlag bool
			for j := range protoDirFindList {
				if strings.Contains(protoGoFileString, protoDirFindList[j]) {
					replaceFlag = true
					protoGoFileString = strings.ReplaceAll(protoGoFileString, protoDirFindList[j], protoDirReplaceList[j])
				}
			}

			if replaceFlag {
				err = os.Remove(protoGoList[i])
				if err != nil {
					log.Fatalln("> protoc_api_import failure:", err)
				}

				protoGoNewFile, err := os.Create(protoGoList[i])
				if err != nil {
					log.Fatalln("> protoc_api_import failure:", err)
				}
				defer protoGoNewFile.Close()

				_, err = protoGoNewFile.WriteString(protoGoFileString)
				if err != nil {
					log.Fatalln("> protoc_api_import failure:", err)
				}
			}
		}(i)
	}
	wg.Wait()

	fmt.Println("> protoc_api_import completed")
}
