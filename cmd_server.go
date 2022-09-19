/* Accept commands from the dxfuse_tools program. The only command
* right now is sync, but this is the place to implement additional
* ones to come in the future.
 */
package dxfuse

import (
	"errors"
	"fmt"
	"log"
	"net"
	"net/rpc"

	"golang.org/x/sync/semaphore"
)

type CmdServer struct {
	options Options
	sybx    *SyncDbDx
	inbound *net.TCPListener
}

// A separate structure used for exporting through RPC
type CmdServerBox struct {
	cmdSrv *CmdServer
}

func NewCmdServer(options Options, sybx *SyncDbDx) *CmdServer {
	cmdServer := &CmdServer{
		options: options,
		sybx:    sybx,
		inbound: nil,
	}
	return cmdServer
}

// write a log message, and add a header
func (cmdSrv *CmdServer) log(a string, args ...interface{}) {
	LogMsg("CmdServer", a, args...)
}

func GetFreePort() (int, error) {
        addr, err := net.ResolveTCPAddr("tcp", "localhost:0")
        if err != nil {
                return 0, err
        }
 
        l, err := net.ListenTCP("tcp", addr)
        if err != nil {
                return 0, err
        }
        defer l.Close()
        return l.Addr().(*net.TCPAddr).Port, nil
}

func (cmdSrv *CmdServer) Init() {
	free_port, err := GetFreePort()
	if free_port == 0 {
		log.Fatal(err)
	}
	cmdSrv.inbound = free_port

	cmdSrvBox := &CmdServerBox{
		cmdSrv: cmdSrv,
	}
	rpc.Register(cmdSrvBox)
	go rpc.Accept(free_port)

	cmdSrv.log("started command server, accepting external commands")
}

func (cmdSrv *CmdServer) Close() {
	cmdSrv.inbound.Close()
}

// Only allow one sync operation at a time
var sem = semaphore.NewWeighted(1)

// Note: all export functions from this module have to have this format.
// Nothing else will work with the RPC package.
func (box *CmdServerBox) GetLine(arg string, reply *bool) error {
	cmdSrv := box.cmdSrv
	cmdSrv.log("Received line %s", arg)
	switch arg {
	case "sync":
		// Error out if another sync operation has been run by the cmd client
		// https://stackoverflow.com/questions/45208536/good-way-to-return-on-locked-mutex-in-go
		if !sem.TryAcquire(1) {
			cmdSrv.log("Rejecting sync operation as another is already runnning")
			return errors.New("another sync operation is already running")
		}
		defer sem.Release(1)
		cmdSrv.sybx.CmdSync()
	default:
		cmdSrv.log("Unknown command")
	}

	*reply = true
	return nil
}
