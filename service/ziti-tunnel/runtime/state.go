package runtime

import (
	"bufio"
	"encoding/json"
	"fmt"
	"golang.zx2c4.com/wireguard/tun"
	"golang.zx2c4.com/wireguard/windows/tunnel/winipcfg"
	"net"
	"os"
	"strconv"
	"time"
	"wintun-testing/cziti"

	"wintun-testing/ziti-tunnel/config"
	"wintun-testing/ziti-tunnel/dto"
	"wintun-testing/ziti-tunnel/idutil"
)

type TunnelerState struct {
	Active     bool
	Duration   int64
	Identities []*dto.Identity
	IpInfo     *TunIpInfo `json:"IpInfo,omitempty"`

	tun     *tun.Device
	tunName string
}

type TunIpInfo struct {
	Ip     string
	Subnet string
	MTU    uint16
	DNS    string
}

func (t *TunnelerState) RemoveByFingerprint(fingerprint string) {
	log.Debugf("removing fingerprint: %s", fingerprint)
	if index, _ := t.Find(fingerprint); index < len(t.Identities) {
		t.Identities = append(t.Identities[:index], t.Identities[index+1:]...)
	}
}

func (t *TunnelerState) Find(fingerprint string) (int, *dto.Identity) {
	for i, n := range t.Identities {
		if n.FingerPrint == fingerprint {
			return i, n
		}
	}
	return len(t.Identities), nil
}

func (t *TunnelerState) RemoveByIdentity(id dto.Identity) {
	t.RemoveByFingerprint(id.FingerPrint)
}

func (t *TunnelerState) FindByIdentity(id dto.Identity) (int, *dto.Identity) {
	return t.Find(id.FingerPrint)
}

func SaveState(s *TunnelerState) {
	// overwrite file if it exists
	_ = os.MkdirAll(config.Path(), 0644)

	cfg, err := os.OpenFile(config.File(), os.O_RDWR|os.O_CREATE|os.O_TRUNC, 0644)
	if err != nil {
		panic(err)
	}
	w := bufio.NewWriter(bufio.NewWriter(cfg))
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	s.IpInfo = nil
	_ = enc.Encode(s)
	_ = w.Flush()

	err = cfg.Close()
	if err != nil {
		panic(err)
	}
	log.Debug("state saved")
}

func (t *TunnelerState) Clean() TunnelerState {
	var d int64
	if t.Active {
		now := time.Now()
		dd := now.Sub(TunStarted)
		d = dd.Milliseconds()
	} else {
		d = 0
	}

	rtn := TunnelerState{
		Active:     t.Active,
		Duration:   d,
		Identities: make([]*dto.Identity, len(t.Identities)),
		IpInfo:     t.IpInfo,
	}
	for i, id := range t.Identities {
		log.Debug("returning clean identity: %s", id.Name)
		rtn.Identities[i] = idutil.Clean(*id)
	}

	return rtn
}

func (t *TunnelerState) CreateTun() error {
	if noZiti() {
		log.Warnf("NOZITI set to true. this should be only used for debugging")
		return nil
	}

	log.Infof("creating TUN device: %s", TunName)
	tunDevice, err := tun.CreateTUN(TunName, 64*1024)
	if err == nil {
		t.tun = &tunDevice
		tunName, err2 := tunDevice.Name()
		if err2 == nil {
			t.tunName = tunName
		}
	} else {
		return fmt.Errorf("error creating TUN device: (%v)", err)
	}

	if name, err := tunDevice.Name(); err == nil {
		log.Debugf("created TUN device [%s]", name)
	}

	nativeTunDevice := tunDevice.(*tun.NativeTun)
	luid := winipcfg.LUID(nativeTunDevice.LUID())
	ip, ipnet, err := net.ParseCIDR(fmt.Sprintf("%s/%d", Ipv4ip, Ipv4mask))
	if err != nil {
		return fmt.Errorf("error parsing CIDR block: (%v)", err)
	}
	log.Debugf("setting TUN interface address to [%s]", ip)
	err = luid.SetIPAddresses([]net.IPNet{{ip, ipnet.Mask}})
	if err != nil {
		return fmt.Errorf("failed to set IP address: (%v)", err)
	}

	dnsServers := []net.IP{
		net.ParseIP(Ipv4dns).To4(),
		net.ParseIP(Ipv6dns),
	}
	err = luid.AddDNS(dnsServers)
	if err != nil {
		return fmt.Errorf("failed to add DNS address: (%v)", err)
	}
	dns, err := luid.DNS()
	if err != nil {
		return fmt.Errorf("failed to fetch DNS address: (%v)", err)
	}
	log.Debugf("dns servers set to = %s", dns)

	log.Infof("routing destination [%s] through [%s]", *ipnet, ipnet.IP)
	err = luid.SetRoutes([]*winipcfg.RouteData{{*ipnet, ipnet.IP, 0}})
	if err != nil {
		return err
	}
	log.Info("routing applied")

	cziti.DnsInit(Ipv4ip, 24)
	cziti.Start()
	_, err = cziti.HookupTun(tunDevice, dns)
	if err != nil {
		panic(err)
	}
	return nil
}

func (t *TunnelerState) Close() {
	if t.tun != nil {
		log.Warn("TODO: actually close the tun - or disable all the identities etc.")
		/*
			cziti.Stop()
		*/
		tu := *t.tun
		err := tu.Close()
		if err != nil {
			log.Fatalf("problem closing tunnel!")
		}
	}
}

func (t *TunnelerState) LoadIdentity(id *dto.Identity) {
	if !noZiti() {
		if id.Connected {
			log.Warnf("id [%s] already connected", id.FingerPrint)
			return
		}
		log.Infof("loading identity %s with fingerprint %s", id.Name, id.FingerPrint)
		ctx := cziti.LoadZiti(id.Path())
		id.Connected = true
		if ctx == nil {
			log.Warnf("connecting to identity with fingerprint [%s] did not error but no context was returned", id.FingerPrint)
			return
		}
		log.Infof("successfully loaded %s@%s", ctx.Name(), ctx.Controller())

		log.Debug("name changed from %s to %s", id.Name, ctx.Name())
		id.Name = ctx.Name()

		if ctx.Services != nil {
			log.Debug("ranging over services...")
			id.Services = make([]*dto.Service, 0)
			for _, svc := range *ctx.Services {
				id.Services = append(id.Services, &dto.Service{
					Name:     svc.Name,
					HostName: svc.InterceptHost,
					Port:     uint16(svc.InterceptPort)})
			}
		} else {
			log.Warnf("no services to load for service name: %s", ctx.Name())
		}
	} else {
		log.Warnf("NOZITI set to true. this should be only used for debugging")
	}
}

func noZiti() bool {
	v, _ := strconv.ParseBool(os.Getenv("NOZITI"))
	return v
}