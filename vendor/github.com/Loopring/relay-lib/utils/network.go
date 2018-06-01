package utils

import "net"

func GetLocalIp() string {
	var res = "unknown"
	addrs, err := net.InterfaceAddrs()
	if err != nil {
		return res
	}
	for _, a := range addrs {
		if ipnet, ok := a.(*net.IPNet); ok && !ipnet.IP.IsLoopback() {
			if ipnet.IP.To4() != nil {
				res = ipnet.IP.To4().String()
			}
		}
	}
	return res
}