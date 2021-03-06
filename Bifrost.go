/*
Copyright [2018] [jc3wish]

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/
package main

import (
	"log"

	"github.com/Bifrost/server"
	"github.com/Bifrost/toserver"
	"github.com/Bifrost/manager"
	"github.com/Bifrost/config"
	"flag"
	"os"
	"os/signal"
	"syscall"
	"time"
	"encoding/json"
	"io"
	"sync"
	"io/ioutil"
)

type recovery struct {
	ToServer *json.RawMessage
	DbInfo *json.RawMessage
}
type recoveryData struct {
	ToServer interface{}
	DbInfo interface{}
}

var l sync.Mutex

var DataFile string
var DataTmpFile string

func main() {
	defer func() {
		server.StopAllChannel()
		doSaveDbInfo()
	}()
	DataFile  = ""
	DataTmpFile = ""
	log.Println("Bifrost start...")
	log.Println("version:",config.VERSION)
	BifrostConfigFile := flag.String("config", "Bifrost.ini", "Bifrost config file path")
	flag.Parse()
	config.LoadConf(*BifrostConfigFile)

	initLog()

	dataDir := config.GetConfigVal("Bifrostd","data_dir")
	if dataDir == ""{
		log.Println("config [ Bifrostd data_dir ] not be empty")
		os.Exit(1)
	}
	os.MkdirAll(dataDir, 0777)
	DataFile = dataDir+"/db.Bifrost"
	DataTmpFile = dataDir+"/db.Bifrost.tmp"

	doRecovery()
	IpAndPort := config.GetConfigVal("Bifrostd","listen")
	if IpAndPort == ""{
		IpAndPort = "0.0.0.0:1036"
	}
	log.Println("Bifrost manager start : ",IpAndPort)

	go TimeSleepDoSaveInfo()
	go manager.Start(IpAndPort)
	ListenSignal()
}

func initLog(){
	log_dir := config.GetConfigVal("Bifrostd","log_dir")
	if log_dir == ""{
		log.Println("no config [ Bifrostd log_dir ] ")
		return
	}
	os.MkdirAll(log_dir,0777)
	t := time.Now().Format("2006-01-02")
	LogFileName := log_dir+"/Bifrost_"+t+".log"
	f, err := os.OpenFile(LogFileName, os.O_CREATE|os.O_RDWR|os.O_APPEND, 0777) //打开文件
	if err != nil{
		log.Println("log init error:",err, " Bifrost.go:76")
	}
	log.SetOutput(f)
}

func TimeSleepDoSaveInfo(){
	for {
		time.Sleep(5 * time.Second)
		doSaveDbInfo()
		}
}

func doSaveDbInfo(){
	l.Lock()
	defer func(){
		l.Unlock()
		if err :=recover();err!=nil{
			log.Println(err)
		}
	}()
	data := recoveryData{
		ToServer:toserver.SaveToServerData(),
		DbInfo:server.SaveDBInfoToFileData(),
	}
	b,_:= json.Marshal(data)
	f, err2 := os.OpenFile(DataTmpFile, os.O_CREATE|os.O_RDWR, 0777) //打开文件
	if err2 !=nil{
		log.Println("open file error:",err2)
		return
	}
	defer f.Close()
	_, err1 := io.WriteString(f, string(b)) //写入文件(字符串)
	if err1 != nil {
		log.Printf("save data to file error:%s, data:%s \r\n",err1,string(b))
		return
	}
	os.Rename(DataTmpFile,DataFile)
}


func doRecovery(){
	fi, err := os.Open(DataFile)
	if err != nil {
		return
	}
	defer fi.Close()
	fd, err := ioutil.ReadAll(fi)
	if err != nil {
		return
	}
	if string(fd) == ""{
		return
	}
	var data recovery
	errors := json.Unmarshal(fd,&data)
	if errors != nil{
		log.Printf("recovery error:%s, data:%s \r\n",errors,string(fd))
	}
	if string(*data.ToServer) != "{}"{
		toserver.Recovery(data.ToServer)
	}
	if string(*data.DbInfo) != "{}"{
		server.Recovery(data.DbInfo)
	}
}

func ListenSignal(){
	signals := make(chan os.Signal, 1)
	signal.Notify(signals, syscall.SIGHUP, syscall.SIGTERM, syscall.SIGINT)
	for sig := range signals {
		if sig == nil{
			continue
		}
		server.StopAllChannel()
		os.Exit(0)
	}
}
