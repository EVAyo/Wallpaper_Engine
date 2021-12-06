package main

import (
    "archive/zip"
    "bytes"
    "context"
    "fmt"
    "io"
    "io/ioutil"
    "log"
    "net/http"
    "os"
    "os/exec"
    "path/filepath"
    "strconv"
    "strings"
    "time"

    "github.com/chromedp/cdproto/network"
    "github.com/chromedp/chromedp"
    "github.com/gogf/gf/encoding/gjson"
    "github.com/gogf/gf/text/gregex"
    "github.com/gogf/gf/text/gstr"
)

// ReDownloadLink 下载失败重下地址 全局存放
var ReDownloadLink = ""
var storageNode=""
var storagePath=""
var file_size="未知大小"
func UnZip(dst, src string) (err error) {
    // 打开压缩文件，这个 zip 包有个方便的 ReadCloser 类型
    // 这个里面有个方便的 OpenReader 函数，可以比 tar 的时候省去一个打开文件的步骤
    zr, err := zip.OpenReader(src)
    defer zr.Close()
    if err != nil {
        return
    }

    // 如果解压后不是放在当前目录就按照保存目录去创建目录
    if dst != "" {
        if err := os.MkdirAll(dst, 0755); err != nil {
            return err
        }
    }

    // 遍历 zr ，将文件写入到磁盘
    for _, file := range zr.File {
        path := filepath.Join(dst, file.Name)

        // 如果是目录，就创建目录
        if file.FileInfo().IsDir() {
            if err := os.MkdirAll(path, file.Mode()); err != nil {
                return err
            }
            // 因为是目录，跳过当前循环，因为后面都是文件的处理
            continue
        }

        // 获取到 Reader
        fr, err := file.Open()
        if err != nil {
            return err
        }

        // 创建要写出的文件对应的 Write
        fw, err := os.OpenFile(path, os.O_CREATE|os.O_RDWR|os.O_TRUNC, file.Mode())
        if err != nil {
            return err
        }

        n, err := io.Copy(fw, fr)
        if err != nil {
            return err
        }

        // 将解压的结果输出
        fmt.Printf("成功解压 %s ，共写入了 %d 个字符的数据\n", path, n)

        // 因为是在循环中，无法使用 defer ，直接放在最后
        // 不过这样也有问题，当出现 err 的时候就不会执行这个了，
        // 可以把它单独放在一个函数中，这里是个实验，就这样了
        fw.Close()
        fr.Close()
    }
    return nil
}

// 下载进度
type WriteCounter struct {
    Total uint64
}

func (wc *WriteCounter) Write(p []byte) (int, error) {
    n := len(p)
    wc.Total += uint64(n)
    wc.PrintProgress()
    return n, nil
}

// 下载进度
func (wc WriteCounter) PrintProgress() {
    fmt.Printf("\r%s", strings.Repeat(" ", 35))
    // Format to a string by passing the number and it's base.
    // fmt.Sprintf("%.2f",float64(sizeIntVar)/1048576 )+"M"
    fmt.Printf("\r正在下载... %sM/ %s", fmt.Sprintf("%.2f",float64(wc.Total)/1048576),file_size)
}

func DownloadFile(filepath string, url string) error {
    out, err := os.Create(filepath + ".tmp")
    if err != nil {
        return err
    }
    resp, err := http.Get(url)
    if err != nil {
        out.Close()
        return err
    }
    defer resp.Body.Close()
    counter := &WriteCounter{}
    if _, err = io.Copy(out, io.TeeReader(resp.Body, counter)); err != nil {
        out.Close()
        return err
    }
    fmt.Print("\n")
    out.Close()
    if err = os.Rename(filepath+".tmp", filepath); err != nil {
        return err
    }
    return nil
}

// 下载及解压
func downloadAndUnzip(url string, fileID string ) {

    fmt.Println("Download Started   " + url)

    err := DownloadFile("./"+fileID+".zip", url)
    if err != nil {
        panic(err)
    }

    fmt.Println("下载完成    " + url)
    // 文件下载 后通过大小判断 是否失败
    fi, err := os.Stat("./" + fileID + ".zip")
    if err == nil {
        fmt.Println("校验下载的文件大小为",fmt.Sprintf("%.2f",float64( fi.Size())/1048576),"M")
    }
    filesize, _ := strconv.ParseInt(fmt.Sprint(fi.Size()), 10, 64)
    // progress, _ = strconv.ParseInt(j.GetString(uuid+".progress"), 10, 64)
    // 如果压缩包小于1M 就不解压
    if filesize < int64(5120) {
        log.Fatalln("文件下载出错（小于512k），请重新下载.....")
        var exitScan string
        _, _ = fmt.Scan(&exitScan)
    }

    // 解压
    if err := UnZip("./projects/defaultprojects/"+fileID, "./"+fileID+".zip"); err != nil {
        log.Fatalln(err)
    }
    err = os.Remove("./" + fileID + ".zip") //删除残留 刚才下载并且解压的的zip

    if err != nil {
        log.Fatalln(err)
    }
}

//监听
func listenForMain(APIUrl string, ReDownLink string) {
    // 等待用户输入
    var Link string

    // fmt.Println("当前平台  " + runtime.GOOS)
    if ReDownLink == "" {
        for {
            fmt.Println("请输入包含ID的连接(可 鼠标右键粘贴)：")
            //当程序只是到fmt.Scanln(&name)程序会停止执行等待用户输入
            fmt.Scanln(&Link)
//Link="https://steamcommunity.com/sharedfiles/filedetails/?id=2650911143&searchtext="
            //Link = "https://steamcommunity.com/sharedfiles/filedetails/?id=2332307710&searchtext="
            //ReDownloadLink = Link
            if !gstr.ContainsI(Link, "https://") {
                fmt.Println("不是正确的https  ID连接，例如 https://steamcommunity.com/sharedfiles/filedetails/?id=2309314482")
                continue
            }
            if !gstr.ContainsI(Link, "?id=") {
                fmt.Println("连接不包含ID，例如 https://steamcommunity.com/sharedfiles/filedetails/?id=2309314482")
                continue
            }
            break
        }
    } else {
        // 先将变量放入ReDownloadLink 节约代码 再将ReDownloadLink 还原到默认值。
        fmt.Println("")
        fmt.Println("=======即将开始重新下载============")
        time.Sleep(time.Second * 6)
        Link = ReDownLink
        ReDownloadLink = ""
    }

    fileID, _ := gregex.MatchString(`id=\d+`, Link)

    fileID, _ = gregex.MatchString(`\d+`, fileID[0])
    fmt.Println("下载的连接的ID是" + fileID[0])
    rawStr :="{\"publishedFileId\":"+fileID[0]+",\"collectionId\":null,\"hidden\":false,\"downloadFormat\":\"raw\",\"autodownload\":false}"
    //rawStr := "{" + "\"publishedFileId\":" + fileID[0] + "," + "\"collectionId\":null,\"extract\":true,\"hidden\":false,\"direct\":false,\"autodownload\":false" + "}"
    var jsonStr = []byte(rawStr)
    r, e := http.NewRequest("POST", APIUrl+"download/request", bytes.NewBuffer(jsonStr))

    client := &http.Client{}
    resp, err := client.Do(r)
    if err != nil {
        panic(err)
    }
    defer resp.Body.Close()

    if e != nil {
        panic(e)
    } else {
        // {"uuid":"734f5478-7b66-49df-a6a0-ddbdf4106d61"}

        s, _ := ioutil.ReadAll(resp.Body) //把  body 内容读入字符串 s
        fmt.Println(string(s))
        var uuid string
        if j, err := gjson.DecodeToJson(string(s)); err != nil {
            panic(err)
        } else {
            uuid = j.GetString("uuid")
        }
        // 判断是否完成 等进度是100 以上 200 这种才开始结束
        //https://api_01.steamworkshopdownloader.io/api/download/status
        // {"uuids":["6df0dc8a-4ccd-4fd2-8532-3c933df4dc80"]}
        var progress int64 = 0

        for progress < int64(100) {
            time.Sleep(time.Second * 1)
            rawStr = "{\"uuids\":[\"" + uuid + "\"]}"
            var jsonStr = []byte(rawStr)
            r, e = http.NewRequest("POST", APIUrl+"download/status", bytes.NewBuffer(jsonStr))

            client = &http.Client{}

            resp, err := client.Do(r)
            if err != nil {
                panic(err)
            }
            defer resp.Body.Close()

            s, _ = ioutil.ReadAll(resp.Body) //把  body 内容读入字符串 s
            fmt.Println(string(s))
            j, err := gjson.DecodeToJson(string(s))
            if err != nil {
                panic(err)
            }
            progress, _ = strconv.ParseInt(j.GetString(uuid+".progress"), 10, 64)

            if strings.Index(j.GetString(uuid+".progressText"), "failed") != -1 {
                // 服务端下载失败
                fmt.Println("服务器端下载失败  稍后后重试。  ")
                fmt.Println("请复制原始Link 开始重新下载（即将重试）   " + Link)

                ReDownloadLink = Link
                break //跳出 上级for循环   for progress < int64(150) {
            }
            fmt.Println("服务器下载进度    " + strconv.FormatInt(progress, 10))
            // {"820a3912-91f3-4174-9edd-40676a1559f4":{"age":76,"status":"error","progress":0,"progressText":"download failed: no steam client available, try again in a minute","downloadError":"never transmitted"}}

       if progress >= int64(100){
           r, e = http.NewRequest("POST", APIUrl+"download/status", bytes.NewBuffer(jsonStr))

           client = &http.Client{}

           resp, err := client.Do(r)
           if err != nil {
               panic(err)
           }
           defer resp.Body.Close()
           s, _ = ioutil.ReadAll(resp.Body) //把  body 内容读入字符串 s
           fmt.Println(string(s))
           j, err := gjson.DecodeToJson(string(s))
           if err != nil {
               panic(err)
           }
           storageNode=j.GetString(uuid+".storageNode")
           storagePath=j.GetString(uuid+".storagePath")
       }
        }
        // 如果服务器没有下载失败，则开始下载
        if ReDownloadLink == "" {
// https://node03.steamworkshopdownloader.io/prod//storage/784150/2002476771/1605174013/2002476771_road_sign_pack.raw.download.zip?uuid=18e11061-dd7c-4bad-a926-4731c36f6dec
            fileDownload := "https://"+storageNode + "/prod/storage/"+storagePath+"?uuid="+uuid
            //       var jsonStr =
            r, e = http.NewRequest("POST", APIUrl+"details/file", bytes.NewBuffer([]byte("["+fileID[0]+"]")))

            client = &http.Client{}

            resp, err := client.Do(r)
            if err != nil {
                panic(err)
            }
            defer resp.Body.Close()
            s, _ = ioutil.ReadAll(resp.Body) //把  body 内容读入字符串 s
            fmt.Println(string(s))
            j, err := gjson.DecodeToJson(string(s))
            if err != nil {
                fmt.Println(err)

            }


            size:=j.GetString("0.file_size")// 1048576
            sizeIntVar, err := strconv.Atoi(size)
            if err != nil {
                file_size="获取大小出错"
            }else {

                file_size=fmt.Sprintf("%.2f",float64(sizeIntVar)/1048576 )+"M"
            }
            downloadAndUnzip(fileDownload, fileID[0])
        }
    }

}



func Decimal(value float64) float64 {
    value, _ = strconv.ParseFloat(fmt.Sprintf("%.2f", value), 64)
    return value
}

func main() {

    // 判断版本更新

    // 判断是否在正确的文件夹下
    _, err := os.Lstat("./wallpaper64.exe")
    if err != nil {
        fmt.Println("当前目录下没有  wallpaper64.exe ，请将本程序放入 wallpaper64.exe 同目录下运行。")
        var exitScan string
        _, _ = fmt.Scan(&exitScan)
        os.Exit(1)
    }
    // 友情提示
    println("Wallpaper Engine资源位置 https://steamcommunity.com/app/431960/workshop/")
    // 自动弹出资源网页，免得手动复制。。。。  懒出新高度
    exec.Command(`cmd`, `/c`, `start`, `https://steamcommunity.com/app/431960/workshop/`).Start()
    println("下载网站 1 https://steamworkshopdownloader.io/")
    println("")
    println("正在使用Chrome获取API......")
    // 浏览主页 获取api
    var APIUrl string
    dir, err := ioutil.TempDir("", "chromedp-example")
    if err != nil {
        panic(err)
    }
    defer os.RemoveAll(dir)

    opts := append(chromedp.DefaultExecAllocatorOptions[:],
        chromedp.DisableGPU,
        chromedp.NoDefaultBrowserCheck,
        chromedp.Flag("headless", true),
        chromedp.Flag("ignore-certificate-errors", true),
        chromedp.Flag("window-size", "400,400"),
        chromedp.UserDataDir(dir),
    )

    allocCtx, cancel := chromedp.NewExecAllocator(context.Background(), opts...)
    defer cancel()

    // also set up a custom logger
    taskCtx, cancel := chromedp.NewContext(allocCtx, chromedp.WithLogf(log.Printf))
    defer cancel()

    // ensure that the browser process is started
    if err := chromedp.Run(taskCtx); err != nil {
        panic(err)
    }
    //listenForNetworkEvent(taskCtx)

    chromedp.ListenTarget(taskCtx, func(ev interface{}) {
        switch ev := ev.(type) {

        case *network.EventResponseReceived:
            resp := ev.Response
            if len(resp.Headers) != 0 {
                // log.Printf("received headers: %s", resp.Headers)

                if strings.Index(resp.URL, "/download/status") != -1 {
                    fmt.Println("找到API啦！！  " + resp.URL)

                    RespURL, err := gregex.MatchString(`https://.+/api/`, resp.URL)
                    if err == nil {
                        APIUrl = RespURL[0]
                    } else {
                        fmt.Println("API 提取错误。。 请GitHub联系 " + resp.URL)
                        println("软件开源地址：https://github.com/user1121114685/Wallpaper_Engine")
                        var exitScan string
                        _, _ = fmt.Scan(&exitScan)
                    }
                    cancel()
                }
            }

        }
        // other needed network Event
    })
    chromedp.Run(taskCtx,
        network.Enable(),
        chromedp.Navigate(`https://steamworkshopdownloader.io/`),
        chromedp.WaitVisible(`body`, chromedp.BySearch),
    )
    for {
        listenForMain(APIUrl, ReDownloadLink)
        // 当不需要重下的时候 才开始重启
        if ReDownloadLink == "" {
            // 自动重启壁纸软件
            exec.Command(`taskkill`, `/F`, `/IM`, `wallpaper64.exe`).Run()
            dir, _ := os.Getwd()
            //fmt.Println("当前路径：", dir)
            exec.Command(dir + `\wallpaper64.exe`).Start()
            time.Sleep(time.Second * 2)
            exec.Command(dir + `\wallpaper64.exe`).Start()
            //exec.Command(`start`, dir+`\wallpaper64.exe`).Run()
            println("")
            println("软件开源地址：https://github.com/user1121114685/Wallpaper_Engine")
            println("执行完毕........已重启 Wallpaper Engine （如遇未运行，请手动打开）.....")
            println("")
            println("........本软件可以多开，不需要等等下载完成.....")
            println("")
        }
    }

}