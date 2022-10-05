package main

import (
	"bufio"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"os/exec"
	"os/signal"
	"regexp"
	"strings"
	"time"

	client "github.com/influxdata/influxdb1-client/v2"
)

type Reader interface {
	Read(rc chan []byte)
}

type Writer interface {
	Writer(wc chan *Message)
}

type ReadFromFile struct {
	path string // 读取文件的路径
}

type WriteToInfluxDB struct {
	influxDBsn string // 写入的信息
}

type LogProcess struct {
	rc chan []byte   // 多个goroutine之间的数据同步和通信（channels)
	wc chan *Message // 写入模块同步数据
	// 系统分为三个模块
	// + 实时读取  -- 文件路径
	// + 解析
	// + 写入  -- 写入的时候需要
	read  Reader //接口定义
	write Writer
}

type Message struct {
	// 使用结构体来存储提取出来的监控数据
	TimeLocal time.Time // 时间
	// BytesSent                                              int       // 流量
	SourceIp, SourceLocation, SourceHostInfo, Path, Method, Scheme, Status string // 请求路径
	// UpstreamTime, RequestTime                              float64   // 监控数据
}

func (r *ReadFromFile) Read(rc chan []byte) {
	// 读取模块
	// 打开文件
	f, err := os.Open(r.path)
	if err != nil {
		panic(fmt.Sprintf("open file error:%s", err.Error()))
	}

	// 从文件末尾开始逐行读取文件内容
	f.Seek(0, 2)
	rd := bufio.NewReader(f) // 对f封装，此时rd就具备更多的方法
	for {
		line, err := rd.ReadBytes('\n') // 读取文件内容直到遇见'\n'为止
		if err == io.EOF {
			// 如果读取到末尾，此时应该等待新的
			time.Sleep(500 * time.Microsecond)
			continue
		} else if err != nil {
			panic(fmt.Sprintf("ReadBytes error:%s", err.Error()))
		}
		rc <- line[:len(line)-1] // 数据的流向
		// 去掉最后的换行符，此时我们可以用切片，从前往后，最后一位换行符-1去掉就好了
	}
}

func (l *LogProcess) Process() {
	//解析模块
	r := regexp.MustCompile(`^([\d]{1,3}\.[\d]{1,3}\.[\d]{1,3}\.[\d]{1,3}) - (.*) \[(.*)\] "([^\s]+) ([^\s]+) ([^\s]+?)" ([\d]{3}) ([\d]{1,9}) "([^"]*?)" "([^"]*?)"`)
	/*
		103.72.172.71 - ying [22/Sep/2022:15:51:16 +0800] "GET /images/06.jpg HTTP/1.1" 304 0 "http://aliyun-chaoyue:8901/" "Mozilla/5.0 (Macintosh; Intel Mac OS X 10.15; rv:104.0) Gecko/20100101 Firefox/104.0"
		103.72.172.71
		ying
		22/Sep/2022:15:51:16 +0800
		GET
		/images/06.jpg
		HTTP/1.1
		304
		0
		http://aliyun-chaoyue:8901/
		Mozilla/5.0 (Macintosh; Intel Mac OS X 10.15; rv:104.0) Gecko/20100101 Firefox/104.0
	*/
	for v := range l.rc {
		ret := r.FindStringSubmatch(string(v)) //匹配数据内容，正则括号内容匹配到返回到
		if len(ret) != 11 {                    //正则表达式有十三个括号
			log.Println("FindStringSubmatch fail:", string(v))
			continue //继续下一次匹配
		}

		message := &Message{}
		message.SourceIp = ret[1] // 访问源
		// message.SourceLocation = GetLocation(message.SourceIp) // 访问源所处的地区
		out, err := exec.Command("/usr/bin/nali", ret[1]).Output()
		if err != nil {
			log.Println("error:", err.Error())
		}
		for _, data := range strings.Split(string(out), " ")[1:] {
			message.SourceLocation += data // 访问源所处的地区
		}
		message.Method = ret[4]
		message.Path = ret[5]   //此时可以直接从结构体中取到path
		message.Scheme = ret[6] //HTTP/1.0协议可以直接赋值给mess
		message.Status = ret[7]
		message.SourceHostInfo = ret[10] // 源主机信息

		l.wc <- message //data是byte类型，需要转化为string类型
	}
}

func (w *WriteToInfluxDB) Writer(wc chan *Message) {
	//写入模块
	infSli := strings.Split(w.influxDBsn, "@") //使用@做切割

	// Create a new HTTPClient
	c, err := client.NewHTTPClient(client.HTTPConfig{
		Addr:     infSli[0], //地址
		Username: infSli[1], //用户名
		Password: infSli[2], //密码
	})
	if err != nil {
		log.Fatal("influxdb 连接失败， err:", err)
	}
	defer c.Close()

	// Create a new point batch
	bp, err := client.NewBatchPoints(client.BatchPointsConfig{
		Database:  infSli[3],
		Precision: infSli[4],
	})
	if err != nil {
		log.Fatal("Create a new point batch, error:", err)
	}

	for v := range wc {
		// 循环的写入数据
		// 搜索条件
		tags := map[string]string{"Path": v.Path, "Method": v.Method, "Scheme": v.Scheme, "Status": v.Status}

		// 需要展示的值
		fields := map[string]interface{}{
			"SourceIp":       v.SourceIp,
			"SourceLocation": v.SourceLocation,
			"SourceHostInfo": v.SourceHostInfo,
		}

		pt, err := client.NewPoint(infSli[5], tags, fields, time.Now()) //创建Influxdb字段
		if err != nil {
			log.Fatal("client.NewPoint err = ", err)
		}
		bp.AddPoint(pt)

		// Write the batch
		if err := c.Write(bp); err != nil {
			log.Fatal("Write the batch， error:", err)
		}

		// Close client resources
		if err := c.Close(); err != nil {
			log.Fatal("Close client resources, error: ", err)
		}

		log.Println("write success") //如果写入成功就打印日志
	}
}

func main() {
	var path, influxDsn string
	flag.StringVar(&path, "path", "./access.log", "read file path") //"帮助信息"
	flag.StringVar(&influxDsn, "influxDsn", "http://172.20.10.111:8086@root@insur132@myself@s@nginx_private", "influx data source")

	flag.Parse() //解析参数
	r := &ReadFromFile{
		path: path,
	}
	w := &WriteToInfluxDB{
		influxDBsn: influxDsn,
	}
	logprocess := &LogProcess{
		rc:    make(chan []byte),
		wc:    make(chan *Message),
		read:  r,
		write: w,
	}

	//使用goroutinue提高程序的性能
	go logprocess.read.Read(logprocess.rc)    //调用读取模块
	go logprocess.Process()                   //调用解析模块
	go logprocess.write.Writer(logprocess.wc) //调用写入模块

	// 等待中断信号以优雅地关闭服务器（设置 5 秒的超时时间）
	quit := make(chan os.Signal)
	signal.Notify(quit, os.Interrupt)
	<-quit
	log.Println("Server Shutdown")
}

// 获取外网ip地址
func GetLocation(ip string) string {
	if ip == "127.0.0.1" || ip == "localhost" {
		return "内部IP"
	}
	resp, err := http.Get("https://restapi.amap.com/v3/ip?ip=" + ip + "&key=f9899f63beeaa5047720b4677b995c80")
	if err != nil {
		panic(err)
	}
	defer resp.Body.Close()
	s, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		log.Println("Read Resp Body error:", err)
	}
	fmt.Println("Resp.Body信息：", string(s))
	m := make(map[string]string)
	err = json.Unmarshal(s, &m)
	if err != nil {
		fmt.Println("Umarshal failed:", err)
	}
	if m["province"] == "" {
		return "未知位置"
	}
	return m["province"] + "-" + m["city"]
}
