package main

//go:generate go-bindata-assetfs asset/...

import (
	"fmt"
	"net"
	"net/http"
	"os"
	"time"

	"github.com/alexcesaro/log"
	"github.com/alexcesaro/log/golog"

	"github.com/xiqingping/golibs/thrustrpc"

	thrustwin "github.com/miketheprogrammer/go-thrust/lib/bindings/window"
	thrustcmd "github.com/miketheprogrammer/go-thrust/lib/commands"
	"github.com/miketheprogrammer/go-thrust/thrust"
)

func main() {
	logger := golog.New(os.Stderr, log.Debug)
	//	http.Handle("/", http.FileServer(&assetfs.AssetFS{
	//		Asset:     Asset,
	//		AssetDir:  AssetDir,
	//		AssetInfo: AssetInfo,
	//		Prefix:    "asset/html"}))

	http.Handle("/", http.FileServer(http.Dir("asset/html")))

	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		logger.Error("Listen:", err)
		return
	}

	fmt.Println(ln.Addr().String())

	thrust.SetProvisioner(ThrustProvisioner{})
	thrust.InitLogger()
	thrust.Start()

	win := thrust.NewWindow(thrust.WindowOptions{
		HasFrame: true,
		RootUrl:  fmt.Sprintf("http://%s/", ln.Addr().String()),
	})
	win.Show()
	win.OpenDevtools()
	win.SetTitle("GVRTool")
	win.Focus()

	rpc, err := thrustrpc.NewRpc(win, logger)
	if err != nil {
		panic(err)
	}

	rpc.Register("add", func(arg []int) (int, error) {
		fmt.Println("add for", arg)
		sum := 0
		for _, v := range arg {
			sum += v
		}
		return sum, nil
	})

	_, err = win.HandleEvent("closed", func(er thrustcmd.EventResult, this *thrustwin.Window) {
		thrust.Exit()
	})

	if err != nil {
		fmt.Println(err)
		thrust.Exit()
	}

	go func() {
		counter := uint32(0)
		for {
			time.Sleep(time.Second)
			rpc.Call("setCounter", counter, time.Millisecond*200)
			counter++
		}
	}()

	http.Serve(ln, nil)
}
