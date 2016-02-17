package main

import (
	"bufio"
	"flag"
	"fmt"
	"github.com/miekg/dns"
	"io"
	"net"
	"os"
	"os/signal"
	"strings"
	"sync"
)

type SimpleARecordHandler struct {
	Name string
	Ttl uint32
	V4Addrs []net.IP
	V6Addrs []net.IP
}

func (h *SimpleARecordHandler) ServeDNS(w dns.ResponseWriter, req *dns.Msg) {
	if req.Question[0].Qtype == dns.TypeA && len(h.V4Addrs) > 0 {
		resp := new(dns.Msg)
		resp.SetReply(req)
		for _, addr := range h.V4Addrs {
			resp.Answer = append(resp.Answer, &dns.A {
				Hdr: dns.RR_Header {
					Name: req.Question[0].Name,
					Rrtype: dns.TypeA,
					Class: dns.ClassINET,
					Ttl: h.Ttl,
				},
				A: addr,
			})
		}
		w.WriteMsg(resp)
	} else if req.Question[0].Qtype == dns.TypeAAAA && len(h.V6Addrs) > 0 {
		resp := new(dns.Msg)
		resp.SetReply(req)
		for _, addr := range h.V6Addrs {
			resp.Answer = append(resp.Answer, &dns.AAAA {
				Hdr: dns.RR_Header {
					Name: req.Question[0].Name,
					Rrtype: dns.TypeAAAA,
					Class: dns.ClassINET,
					Ttl: h.Ttl,
				},
				AAAA: addr,
			})
		}
		w.WriteMsg(resp)
	} else {
		dns.HandleFailed(w, req)
		return
	}
}

func makeServers(tpsString string, handler dns.Handler) ([]*dns.Server, error) {
	tps := make(map[string]string)
	for _, v := range strings.Split(strings.Trim(tpsString, "\r\n \t"), ",") {
		v = strings.Trim(v, "\r\n \t")
		pair := strings.SplitN(v, ":", 2)
		tp := ""
		addr := ""
		if len(pair) == 2 {
			tp, addr = pair[0], pair[1]
		} else {
			tp = pair[0]
			addr = "0.0.0.0"
		}
		tps[tp] = addr
	}
	servers := make([]*dns.Server, 0, len(tps))
	for tp, addr := range tps {
		servers = append(servers, &dns.Server { Addr: addr, Net: tp, Handler: handler })
	}
	return servers, nil
}

func makeHandler(fileName string, ttl uint32) (*dns.ServeMux, error) {
	f, err := os.Open(fileName)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	rdr := bufio.NewReader(f)
	ln := 0
	pairs := make(map[string]*SimpleARecordHandler)
	retval := dns.NewServeMux()
	for {
		ln += 1
		_l, isPfx, err := rdr.ReadLine()
		if err != nil {
			if err == io.EOF {
				break
			}
			return nil, fmt.Errorf("%s:%d %T - %s", fileName, ln, err, err.Error())
		}
		if isPfx {
			return nil, fmt.Errorf("%s:%d line too long", fileName, ln)
		}
		// assuming the file is encoded in UTF-8
		l := string(_l)
		pair := strings.SplitN(l, "\t", 2)
		if len(pair) != 2 {
			return nil, fmt.Errorf("%s:%d invalid record", fileName, ln)
		}
		ip := net.ParseIP(pair[1])
		if ip == nil {
			return nil, fmt.Errorf("%s:%d invalid IP address format", fileName, ln)
		}
		h, ok := pairs[pair[0]]
		if !ok {
			h = &SimpleARecordHandler { Name: pair[0], Ttl: ttl, V4Addrs: make([]net.IP, 0, 1), V6Addrs: make([]net.IP, 0, 1) }
			pairs[pair[0]] = h
			retval.Handle(pair[0], h)
		}
		if ip.To4() == nil {
			h.V6Addrs = append(h.V6Addrs, ip)
		} else {
			h.V4Addrs = append(h.V4Addrs, ip)
		}
	}
	return retval, nil
}

func main() {
	tpsString := flag.String("listen", "udp:127.0.0.1:8053", "tp to use [{tcp,tcp4,tcp6,udp,udp4,udp6}:addr]")
	ttl := flag.Int("ttl", 300, "TTL value")
	flag.Parse()
	if flag.NArg() != 1 {
		flag.PrintDefaults()
		os.Exit(255)
	}
	arg := flag.Args()[0]
	serveMux, err := makeHandler(arg, uint32(*ttl))
	if err != nil {
		fmt.Fprintf(os.Stderr, "%T - %s\n", err, err.Error())
		os.Exit(1)
	}
	servers, err := makeServers(*tpsString, serveMux)
	if err != nil {
		fmt.Fprintf(os.Stderr, "%T - %s\n", err, err.Error())
		os.Exit(1)
	}
	wg := sync.WaitGroup {}
	for i, server := range servers {
		wg.Add(1)
		go func(i int, server *dns.Server) {
			defer wg.Done()
			err := server.ListenAndServe()
			if err != nil {
				fmt.Fprintf(os.Stderr, "%T - %s\n", err, err.Error())
			} else {
				fmt.Fprintf(os.Stderr, "Server #%d terminated.\n", i)
			}
		}(i, server)
	}
	signalChan := make(chan os.Signal, 1)
	signal.Notify(signalChan, os.Kill, os.Interrupt)
	go func() {
		sig := <-signalChan
		fmt.Fprintf(os.Stderr, "Received %s\n", sig.String())
		for i, server := range servers {
			err := server.Shutdown()
			if err != nil {
				fmt.Fprintf(os.Stderr, "Server #%d failed to terminate: %s.\n", i, err.Error())
			}
		}
	}()
	wg.Wait()
}
