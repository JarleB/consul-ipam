package main

import consulapi "github.com/hashicorp/consul/api"
// import "github.com/davecgh/go-spew/spew"
import "fmt"
import "net"
import "os"

/*
This is a basic IP admin tool that uses consul KV to specify IP ranges,
and consul catalog to find used IP address. It allocates IP node addresses
from unused addresses in the range and register name:ip in consul catalog.

DNS A records are immidiately available in Consul DNS when new nodes are registered

*/

func inc(ip net.IP) {
	// Find the next ip in range
	for j := len(ip) - 1; j >= 0; j-- {
		ip[j]++
		if ip[j] > 0 {
			break
		}
	}
}

func populaterange(start string, end string) ( []string) {
	// Return an array of ips in the range between (inclusive) start and end
  s := net.ParseIP(start)
  e := net.ParseIP(end)
  var ips []string
  for ip := s; ! ip.Equal(e); inc(ip) {
    ips = append(ips,ip.String())
    //fmt.Printf("%s\n", ip)
  }
  return ips
}

func difference(a, b []string) []string {
	// Find which strings in a that are not in b
    mb := map[string]bool{}
    for _, x := range b {
        mb[x] = true
    }
    ab := []string{}
    for _, x := range a {
        if _, ok := mb[x]; !ok {
            ab = append(ab, x)
        }
    }
    return ab
}

/* More like intersect ?
func difference(slice1 []string, slice2 []string) ([]string){
    diffStr := []string{}
    m :=map [string]int{}

    for _, s1Val := range slice1 {
        m[s1Val] = 1
    }
    for _, s2Val := range slice2 {
        m[s2Val] = m[s2Val] + 1
    }

    for mKey, mVal := range m {
        if mVal==1 {
            diffStr = append(diffStr, mKey)
        }
    }

    return diffStr
}
*/

func consulclient ( url string ) ( consulapi.Client ) {
  conf := consulapi.DefaultNonPooledConfig()
  conf.Address = url
  conf.Scheme = "http"
  client, err := consulapi.NewClient(conf)
  if err != nil {
    panic(err)
  }
  return *client
}

func used_consulips ( c consulapi.Client ) ( []string ) {
  catalog := c.Catalog()
  services,_,_ := catalog.Services(nil)
  var ips []string
  for k, _ := range services {
      s, _, _ := catalog.Service(k, "", nil)
     address := s[0].ServiceAddress
     if len(address) > 0 {
       ips = append(ips,address)
     }
   }
	nodes,_,_ := catalog.Nodes(nil)
  for _, k := range nodes {
      cn, _, _ := catalog.Node(k.Node, nil)
     address := cn.Node.Address
     if len(address) > 0 {
       ips = append(ips,address)
     }
   }
   return ips
}

func iprange ( c consulapi.Client, prefix string ) ( start, end string ) {
	// Get start and end ip from consul KV
  kv := c.KV()
  startpair, _, _ := kv.Get(prefix + "/start" , nil)
  endpair, _, _   := kv.Get(prefix + "/end" , nil)
  if startpair == nil || endpair == nil {
    fmt.Printf("Could not find start/end in prefix: %s\n",prefix)
    os.Exit(2)
  }
  return string(startpair.Value), string(endpair.Value)
}

func nreg ( name string, ip string, c consulapi.Client ) {
	// Register a node with the specified IP address
	// will be available in dns as $ dig -p 8600 @one-of-the-consul-masters <nodename>.node.<datacenter>.consul
	cat := c.Catalog()
	creg := new(consulapi.CatalogRegistration)
	creg.Node = name
	creg.Address = ip
	_, err := cat.Register(creg, nil)
	if err != nil {
		panic(err)
	}
}

func ndereg (name string, c consulapi.Client ) {
	cat := c.Catalog()
	dereg := new(consulapi.CatalogDeregistration)
	dereg.Node = name
	_, err := cat.Deregister(dereg, nil)
	if err != nil {
		panic(err)
	}
}
/*
// unused
func sreg ( s_name string, s_ip string, n_name string, n_ip string, c consulapi.Client ) {
	cat := c.Catalog()
	srv  :=  new(consulapi.AgentService)
	creg := new(consulapi.CatalogRegistration)
	//var wrops *consulapi.WriteOptions
	//srv.ID = name
	creg.Node = name
	creg.Address = ip
	srv.Service = name
	srv.Address = ip
	//creg.Datacenter = "vilberg"
	//fmt.Printf("%s: %s\n",creg.Node,ip)
	//creg.SkipNodeUpdate = true
	creg.Service = srv
	_, err := cat.Register(creg, nil)
	if err != nil {
		panic(err)
	}
}
 */
func main() {
  if len(os.Args) != 5 {
    fmt.Printf("Usage: %s consul-api-url location-of-iprange-in-kv <node-name> <reg/dereg>]\n", os.Args[0] )
    os.Exit(1)
  }
  url := os.Args[1]
  prefix := os.Args[2]
	nodename := os.Args[3]
	operation := os.Args[4]
  client := consulclient(url)
	if operation == "dereg" {
		ndereg(nodename, client)
		fmt.Printf("%s","deregistered")
 	} else if operation == "reg" {
    start, end := iprange(client, prefix)
    initialips := populaterange(start, end)
    usedips := used_consulips(client)
    availableips := difference(initialips,usedips)
    	// Register node with an available ip in the range
  	nreg (nodename, availableips[0], client)
		fmt.Printf("%s",availableips[0])
 }
}
