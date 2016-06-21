// wangdk project main.go
package main

import (
	"archive/tar"
	"errors"

	"fmt"
	"io"
	"io/ioutil"

	"gitserver/iop/DockerStack/common/mq"
	"os"
	"os/exec"
	"path"
	"strconv"
	"text/template"
	"time"
	"github.com/coreos/etcd/client"
	"golang.org/x/net/context"
)

// 将文件或目录打包成 .tar 文件
// src 是要打包的文件或目录的路径
// dstTar 是要生成的 .tar 文件的路径
// failIfExist 标记如果 dstTar 文件存在，是否放弃打包，如果否，则会覆盖已存在的文件
func Tar(src string, dstTar string, failIfExist bool) (err error) {
	// 清理路径字符串
	src = path.Clean(src)

	// 判断要打包的文件或目录是否存在
	if !Exists(src) {
		return errors.New("要打包的文件或目录不存在：" + src)
	}

	// 判断目标文件是否存在
	if FileExists(dstTar) {
		if failIfExist { // 不覆盖已存在的文件
			return errors.New("目标文件已经存在：" + dstTar)
		} else { // 覆盖已存在的文件
			if er := os.Remove(dstTar); er != nil {
				return er
			}
		}
	}

	// 创建空的目标文件
	fw, er := os.Create(dstTar)
	if er != nil {
		return er
	}
	defer fw.Close()

	// 创建 tar.Writer，执行打包操作
	tw := tar.NewWriter(fw)
	defer func() {
		// 这里要判断 tw 是否关闭成功，如果关闭失败，则 .tar 文件可能不完整
		if er := tw.Close(); er != nil {
			err = er
		}
	}()

	// 获取文件或目录信息
	fi, er := os.Stat(src)
	if er != nil {
		return er
	}

	// 获取要打包的文件或目录的所在位置和名称
	srcBase, srcRelative := path.Split(path.Clean(src))

	// 开始打包
	if fi.IsDir() {
		tarDir(srcBase, srcRelative, tw, fi)
	} else {
		tarFile(srcBase, srcRelative, tw, fi)
	}

	return nil
}

// 因为要执行遍历操作，所以要单独创建一个函数
func tarDir(srcBase, srcRelative string, tw *tar.Writer, fi os.FileInfo) (err error) {
	// 获取完整路径
	srcFull := srcBase + srcRelative

	// 在结尾添加 "/"
	last := len(srcRelative) - 1
	if srcRelative[last] != os.PathSeparator {
		srcRelative += string(os.PathSeparator)
	}

	// 获取 srcFull 下的文件或子目录列表
	fis, er := ioutil.ReadDir(srcFull)
	if er != nil {
		return er
	}

	// 开始遍历
	for _, fi := range fis {
		if fi.IsDir() {
			tarDir(srcBase, srcRelative+fi.Name(), tw, fi)
		} else {
			tarFile(srcBase, srcRelative+fi.Name(), tw, fi)
		}
	}

	// 写入目录信息
	if len(srcRelative) > 0 {
		hdr, er := tar.FileInfoHeader(fi, "")
		if er != nil {
			return er
		}
		hdr.Name = srcRelative

		if er = tw.WriteHeader(hdr); er != nil {
			return er
		}
	}

	return nil
}

// 因为要在 defer 中关闭文件，所以要单独创建一个函数
func tarFile(srcBase, srcRelative string, tw *tar.Writer, fi os.FileInfo) (err error) {
	// 获取完整路径
	srcFull := srcBase + srcRelative

	// 写入文件信息
	hdr, er := tar.FileInfoHeader(fi, "")

	if er != nil {
		return er
	}
	hdr.Name = srcRelative

	if er = tw.WriteHeader(hdr); er != nil {
		return er
	}

	// 打开要打包的文件，准备读取
	fr, er := os.Open(srcFull)
	if er != nil {
		return er
	}
	defer fr.Close()

	// 将文件数据写入 tw 中
	if _, er = io.Copy(tw, fr); er != nil {
		return er
	}
	return nil
}

func UnTar(srcTar string, dstDir string) (err error) {
	// 清理路径字符串
	dstDir = path.Clean(dstDir) + string(os.PathSeparator)

	// 打开要解包的文件
	fr, er := os.Open(srcTar)
	if er != nil {
		return er
	}
	defer fr.Close()

	// 创建 tar.Reader，准备执行解包操作
	tr := tar.NewReader(fr)

	// 遍历包中的文件
	for hdr, er := tr.Next(); er != io.EOF; hdr, er = tr.Next() {
		if er != nil {
			return er
		}

		// 获取文件信息
		fi := hdr.FileInfo()

		// 获取绝对路径
		dstFullPath := dstDir + hdr.Name

		if hdr.Typeflag == tar.TypeDir {
			// 创建目录
			os.MkdirAll(dstFullPath, fi.Mode().Perm())
			// 设置目录权限
			os.Chmod(dstFullPath, fi.Mode().Perm())
		} else {
			// 创建文件所在的目录
			os.MkdirAll(path.Dir(dstFullPath), os.ModePerm)
			// 将 tr 中的数据写入文件中
			if er := unTarFile(dstFullPath, tr); er != nil {
				return er
			}
			// 设置文件权限
			os.Chmod(dstFullPath, fi.Mode().Perm())
		}
	}
	return nil
}

// 因为要在 defer 中关闭文件，所以要单独创建一个函数
func unTarFile(dstFile string, tr *tar.Reader) error {
	// 创建空文件，准备写入解包后的数据
	fw, er := os.Create(dstFile)
	if er != nil {
		return er
	}
	defer fw.Close()

	// 写入解包后的数据
	_, er = io.Copy(fw, tr)
	if er != nil {
		return er
	}

	return nil
}

// 判断档案是否存在
func Exists(name string) bool {
	_, err := os.Stat(name)
	return err == nil || os.IsExist(err)
}

// 判断文件是否存在
func FileExists(filename string) bool {
	fi, err := os.Stat(filename)
	return (err == nil || os.IsExist(err)) && !fi.IsDir()
}

// 判断目录是否存在
func DirExists(dirname string) bool {
	fi, err := os.Stat(dirname)
	return (err == nil || os.IsExist(err)) && fi.IsDir()
}

func ListenEvents() {
	//	listContainrOpetions := docker.ListContainersOptions{}

	//client := common.GetClient()
	//result, _ := client.ListContainers(listContainrOpetions)
	//	client.AddEventListener()
	//	fmt.Println(result)
}
func TestTar() {
	src := "/root/tmp/cloud-agent-python/agent/shell/"
	tarfiledir := "/opt/DockerStack/wangdekui"

	os.MkdirAll(tarfiledir, 0750)
	command := "cd " + tarfiledir + ";" + "cp " + src + "/*" + " . -R ;tar cvf " + tarfiledir + "/temp.tar ."
	cmd := exec.Command("/bin/bash", "-c", command)
	err := cmd.Run()
	if err != nil {
		fmt.Println("tar file failed", err)
	}
}
func EtcdTest() {
	//docker info to etcd
	cfg := client.Config{
		Endpoints:               []string{"http://10.110.17.24:4001"},
		Transport:               client.DefaultTransport,
		HeaderTimeoutPerRequest: time.Second,
	}
	c, err := client.New(cfg)
	if err != nil {
		fmt.Println(err)
	}
	containerIp := "192.168.9.11"
	containerPort := "8080"
	domain := "www.helloworld.com"
	kapi := client.NewKeysAPI(c)
	key := "dockerstack/" + domain
	value := containerIp + ":" + containerPort
	//kapi.CreateInOrder()

	kapi.CreateInOrder(context.Background(), key, value, &client.CreateInOrderOptions{})
	kapi.CreateInOrder(context.Background(), key, "192.168.9.99:8080", &client.CreateInOrderOptions{})

	getOptions := client.GetOptions{}
	resp1, _ := kapi.Get(context.Background(), key, &getOptions)
	fmt.Println(resp1.Node.Key)
	//fmt.Println(resp1.Node.Value)
	fmt.Println(resp1.Node.Nodes[0].Value)
	//fmt.Println("Get is done. Metadata is %q\n", resp1)
}
func TestTemlate() {
	type Inventory struct {
		Material string
		Count    uint
	}

	type ContaierHaproxy struct {
		IP       string
		Port     int64
		HostName string
	}
	type Haproxy struct {
		IP          string
		Port        int64
		ServiceName string
		Containers  []*ContaierHaproxy
	}
	//	var containerInfo []Haproxy
	var containers []*ContaierHaproxy
	for i := 1; i <= 10; i++ {
		container := ContaierHaproxy{
			IP:       strconv.Itoa(i),
			Port:     int64(i),
			HostName: "a",
		}
		containers = append(containers, &container)
	}
	haproxy := Haproxy{IP: "127.0.0.1",
		Port:        int64(6666),
		ServiceName: "test",
		Containers:  containers}

	tmpl, err := template.ParseFiles("/root/goworkspace/src/gitserver/iop/DockerStack/conf/haproxy.cfg.tmpl")
	if err != nil {
		fmt.Println(err)
	}
	var containerInfo []*Haproxy
	containerInfo = append(containerInfo, &haproxy)
	ha := "/root/goworkspace/src/gitserver/iop/DockerStack/conf/haproxy.cfg"
	writer, _ := os.Create(ha)
	err = tmpl.Execute(writer, containerInfo)
	if err != nil {
		fmt.Println(err)
	}

}

func TestMyMqConnection() {
	var mq1 *mq.BaseMq
	mq1 = mq.GetConnection("manager")
	channleContxt := mq.ChannelContext{Exchange: "docker-exchange",
		ExchangeType: "direct",
		RoutingKey:   "docker",
		Reliable:     true,
		Durable:      false}
	for {
		fmt.Println("sending message")
		mq1.Publish(&channleContxt, "helllosaga")
		time.Sleep(10 * time.Second)
	}
}
func TestMyMqConnection1() {
	var mq1 *mq.BaseMq
	mq1 = mq.GetConnection("manager")
	channleContxt := mq.ChannelContext{Exchange: "docker-exchange1",
		ExchangeType: "direct",
		RoutingKey:   "docker1",
		Reliable:     true,
		Durable:      false}
	for {
		fmt.Println("sending message111")
		mq1.Publish(&channleContxt, "helllosaga")
		time.Sleep(10 * time.Second)
	}
}

/*
//test rabbitmq
var (
	uri          = flag.String("uri", "amqp://guest:guest@127.0.0.1:5672/", "AMQP URI")
	exchangeName = flag.String("exchange", "test-exchange", "Durable AMQP exchange name")
	exchangeType = flag.String("exchange-type", "direct", "Exchange type - direct|fanout|topic|x-custom")
	routingKey   = flag.String("key", "test-key", "AMQP routing key")
	body         = flag.String("body", "foobar", "Body of message")
	reliable     = flag.Bool("reliable", true, "Wait for the publisher confirmation before exiting")
)
type struct MqConnection{
	Lock sync.RWMutex
	Connection *amqp.Connection
}
type struct ChannelContext{
    Channel *amqp.Channel
	Exchange string
	ExchangeType string
	RoutingKey string
    Reliable bool
}
var monitorMqConnection *MqConnection
connectionErrorChan := make(chan *amqp.Error)

// One would typically keep a channel of publishings, a sequence number, and a
// set of unacknowledged sequence numbers and loop until the publishing channel
// is closed.
func confirmOne(confirms <-chan amqp.Confirmation) {
	log.Printf("waiting for confirmation of one publishing")

	if confirmed := <-confirms; confirmed.Ack {
		log.Printf("confirmed delivery with delivery tag: %d", confirmed.DeliveryTag)
	} else {
		log.Printf("failed delivery of delivery tag: %d", confirmed.DeliveryTag)
	}
}



func refreshConnectionAndChannel(channelContext *ChannelContext, mqConnection *MqConnection) {
	mqConnection.Lock.lock()
    defer mqConnection.lock.Unlock()
    channel, err := mqConnection.Connection.Channel()
	if err != nil {
    for{
        mqConnection.Connection, err1= amqp.Dial("amqp://guest:guest@127.0.0.1:5672/")
       if err1!=nil{
	     fmt.Println("connect mq error,retry...")
	     time.Sleep(10*time.Second)
        }else{
            mqConnection.Channel=mqConnection.Connection.Channel()
	        break
	    }
      }
	}

	if err := channel.ExchangeDeclare(
		channelContext.Exchange,     // name
		channelContext.ExchangeType, // type
		true,         // durable
		false,        // auto-deleted
		false,        // internal
		false,        // noWait
		nil,          // arguments
	); err != nil {
		fmt.Println("channel exchange deflare failed refreshConnectionAndChannel again")
        time.Sleep(10*time.Second)
		refreshConnectionAndChannel()
	}

	// Reliable publisher confirms require confirm.select support from the
	// connection.
	if channelContext.Reliable {
		log.Printf("enabling publishing confirms.")
		if err := channelContext.Channel.Confirm(false); err != nil {
			fmt.Errorf("Channel could not be put into confirm mode: %s", err)
            time.Sleep(10*time.Second)
		    refreshConnectionAndChannel()
		}
		confirms := channel.NotifyPublish(make(chan amqp.Confirmation, 1))
		defer confirmOne(confirms)
	}
}

func publish(channelContext *ChannelContext, body string) error {
    if channelContext.Channle==nil{
		refreshConnectionAndChannel(channelContext)
	}

	log.Printf("declared Exchange, publishing %dB body (%q)", len(body), body)
	for {
		log.Printf("publish message publishing %dB body (%q)", len(body), body)
		if err = channel.Publish(
			exchange,   // publish to an exchange
			routingKey, // routing to 0 or more queues
			false,      // mandatory
			false,      // immediate
			amqp.Publishing{
				Headers:         amqp.Table{},
				ContentType:     "application/json",
				ContentEncoding: "",
				Body:            []byte(body),
				DeliveryMode:    amqp.Transient, // 1=non-persistent, 2=persistent
				Priority:        0,              // 0-9
				// a bunch of application/implementation-specific fields
			},
		); err != nil {
			return fmt.Errorf("Exchange Publish: %s", err)
		}
		time.Sleep(5 * time.Second)
	}
	return nil
}

listen to the connectionErrorChan
*/
/*func connectionListener() {
	for {
		select {
		case value := <-chan1:
			fmt.Println("read from channel1 value is ", value)

			if value.(amqp.Error) {
             refreshConnection()
             return
			}

		default:
			fmt.Println("nothing form channel1")
		}
		time.Sleep(10 * time.Second)
	}
}*/

func main() {

	go TestMyMqConnection()
	go TestMyMqConnection1()
	for {
		time.Sleep(10 * time.Second)
	}

}
