/*
* NIST-developed software is provided by NIST as a public service. You
* may use, copy and distribute copies of the software in any medium,
* provided that you keep intact this entire notice. You may improve,
* modify and create derivative works of the software or any portion of
* the software, and you may copy and distribute such modifications or
* works. Modified works should carry a notice stating that you changed
* the software and should note the date and nature of any such
* change. Please explicitly acknowledge the National
* Institute of Standards and Technology as the source of the software.

* NIST-developed software is expressly provided “AS IS.” NIST MAKES NO
* WARRANTY OF ANY KIND, EXPRESS, IMPLIED, IN FACT OR ARISING BY
* OPERATION OF LAW, INCLUDING, WITHOUT LIMITATION, THE IMPLIED
* WARRANTY OF MERCHANTABILITY, FITNESS FOR A PARTICULAR PURPOSE,
* NON-INFRINGEMENT AND DATA ACCURACY. NIST NEITHER REPRESENTS NOR
* WARRANTS THAT THE OPERATION OF THE SOFTWARE WILL BE UNINTERRUPTED OR
* ERROR-FREE, OR THAT ANY DEFECTS WILL BE CORRECTED. NIST DOES NOT
* WARRANT OR MAKE ANY REPRESENTATIONS REGARDING THE USE OF THE
* SOFTWARE OR THE RESULTS THEREOF, INCLUDING BUT NOT LIMITED TO THE
* CORRECTNESS, ACCURACY, RELIABILITY, OR USEFULNESS OF THE SOFTWARE.

* You are solely responsible for determining the appropriateness of
* using and distributing the software and you assume all risks
* associated with its use, including but not limited to the risks and
* costs of program errors, compliance with applicable laws, damage to
* or loss of data, programs or equipment, and the unavailability or
* interruption of operation. This software is not intended to be used
* in any situation where a failure could cause risk of injury or
* damage to property. The software developed by NIST employees is not
* subject to copyright protection within the United States.
*/

package main

import (
	"fmt"
	"flag"
	"errors"
	"bufio"
	"os"
	"strings"
	"time"
	"github.com/miekg/dns"
	"gopkg.in/mgo.v2"
	"gopkg.in/mgo.v2/bson"
)

const (
	// DefaultTimeout is default timeout many operation in this program will
	// use.
	DefaultTimeout time.Duration = 5 * time.Second
)

type zoneDnsPosture struct {
	Id string `json:"id" bson:"_id,omitempty"`
	Zname string `json:"zname" bson:"zname"`
	Agency string `json:"agency" bson:"agency"`
	Time string `json:"time" bson:"time"`
    Serial uint32  `json:"serial" bson:"serial"`
	Status string `json:"status" bson:"status"`
    Ksks []uint16 `json:"ksks" bson:"ksks"`
    Zsks []uint16 `json:"zsks" bson: zsks"`
    Algos []uint8 `json:"algos" bson: "algos"`
    NsSet []string `json"nsset" bson: "nssec"`
    DsHash []uint8 `json"dshash" bson:"dshash"`
}

var (
	myRes *dns.Client
	conf  *dns.ClientConfig
	usrName	string
	dbPass	string
	dbUrl string
	dbName string
	inputList string
)

func doQuery(qname string, qtype uint16, validate bool) (*dns.Msg, error) {
	query := new (dns.Msg)
	query.RecursionDesired = true
	query.SetEdns0(4096, validate)
	query.SetQuestion(dns.Fqdn(qname), qtype)
	
	for i := range conf.Servers {
		server := conf.Servers[i]
		r, _, err := myRes.Exchange(query, server+":"+conf.Port)
		if err != nil || r == nil {
			return nil, err
		} else {
			return r, err
		}
	}
	return nil, errors.New("No name server to answer the question")
}


func getNSList(qname string) ([]string) {
	var servers []string
	
	resp,err := doQuery(qname, dns.TypeNS, false)
	if err != nil || resp == nil {
		servers = append(servers, "none")
		return servers
	}
	//NEED TO CHANGE: have the Contains deal with upper/lower case
	for _, aRR := range resp.Answer {
		switch aRR := aRR.(type) {
			case *dns.NS:
				servers = append(servers, aRR.Ns)
		}
	}
	return servers
}

func getSoaSerial(qname string) (uint32) {
	resp,err := doQuery(qname, dns.TypeSOA, false)
	if err != nil || resp == nil {
		return 0
	}
	//NEED TO CHANGE: have the Contains deal with upper/lower case
	for _, aRR := range resp.Answer {
		switch aRR := aRR.(type) {
			case *dns.SOA:
				return aRR.Serial
		}
	}
	return 0
}

func parseConfigFile(filename string) {
	if file, err := os.Open(filename); err == nil {
		defer file.Close()
		
		scanner := bufio.NewScanner(file)
		for scanner.Scan() {
			line := strings.Split(scanner.Text(), "=")
			switch (line[0]) {
				case ("user"):
					usrName = line[1]
				case ("db"):
					dbName = line[1]
				case ("url"):
					dbUrl = line[1]
					fmt.Println("made it to url")
				case ("pass"):
					dbPass = line[1]
				case ("input"):
					inputList = line[1]
			}
		}
	}
}

func main() {
	var err error
	var line []string
	var session *mgo.Session	
	
	//get the arguements
	confFile := flag.String("config", "monitor.conf", "The configuration file")
	flag.Parse()
	
	parseConfigFile(*confFile)

	conf, err = dns.ClientConfigFromFile("/etc/resolv.conf")
	if err != nil || conf == nil {
		fmt.Printf("Cannot initialize the local resolver: %s\n", err)
		os.Exit(1)
	}
	myRes = &dns.Client{
		ReadTimeout: DefaultTimeout,
	}
	
	// open the input file
	if file, err := os.Open(inputList); err == nil {
		// make sure it gets closed
		defer file.Close()
		var zonename string
		//connect to db
		monDBDialInfo := &mgo.DialInfo{
			Addrs: []string{dbUrl},
			Timeout: 60 * time.Second,
			Database: "dns",
			Username: usrName,
			Password: dbPass,
		}
		session, err = mgo.DialWithInfo(monDBDialInfo)
		if (err != nil) {
			fmt.Println("can't connect to db!!\n")
			panic (err.Error())
		}
		session.SetSafe(&mgo.Safe{})
		cur := session.DB("dns").C(dbName)
		index := mgo.Index{
				Key:        []string{"zname"},
				Unique:     true,
				DropDups:   true,
				Background: true,
				Sparse:     true,
		}
		err = cur.EnsureIndex(index)
		if err != nil {
			fmt.Println("Failed to insure index?\n")
			panic(err)
		}
		
		// create a new scanner and read the file line by line
		scanner := bufio.NewScanner(file)
		
		for scanner.Scan() {
			var zoneData zoneDnsPosture
			line = strings.Split(scanner.Text(), ",")
			zonename = dns.Fqdn(line[0])
			zoneData.Zname = zonename
			zoneData.Agency = line[2]
			zoneData.Time = time.Now().String()
            zoneData.NsSet = getNSList(zonename)
			zoneData.Serial = getSoaSerial(zonename)
			resp,err := doQuery(zonename, dns.TypeDNSKEY, true)
			if err == nil && resp != nil {	
				if (resp.Rcode != dns.RcodeNameError) {
					if (resp.MsgHdr.AuthenticatedData) {
						zoneData.Status = "valid"
					} else {	
						zoneData.Status = "island"
					}
					if (resp.Rcode == dns.RcodeServerFailure) {
						//could be validation error - redo with CD bit set
						zoneData.Status = "bogus"
						resp, err = doQuery(zonename, dns.TypeDNSKEY, false)
					} 						
					if (len(resp.Answer) > 0) {	
						for _, aRR := range resp.Answer {
							switch aRR := aRR.(type) {
								case *dns.DNSKEY:
									if (aRR.Flags == 256) {
										zoneData.Zsks = append(zoneData.Zsks, aRR.KeyTag())
										zoneData.Algos = append(zoneData.Algos, aRR.Algorithm)
									}
									if (aRR.Flags == 257) {
										zoneData.Ksks = append(zoneData.Ksks, aRR.KeyTag())
										zoneData.Algos = append(zoneData.Algos, aRR.Algorithm)
									}
							}
						}	 
						//now get the DS RRset hashes
						resp,err = doQuery(zonename, dns.TypeDS, false)
						if err == nil || resp != nil {
							if (len(resp.Answer) > 0) {
								for _, aRR := range resp.Answer {
									switch aRR := aRR.(type) {
										case *dns.DS:
											zoneData.DsHash = append(zoneData.DsHash, aRR.DigestType)
									}
								} 
							} else {  //no DS RRs - island
								zoneData.DsHash = append(zoneData.DsHash, 0)
								zoneData.Status = "island"
							}
						}
					} else {  //it is unsigned
						zoneData.Status = "unsigned"
						zoneData.Zsks = append(zoneData.Zsks, 0)
						zoneData.Ksks = append(zoneData.Ksks, 0)
						zoneData.Algos = append(zoneData.Algos, 0)
					}
					//now put it in the database
					nameKey := bson.M{"zname": zonename }
					_, err = cur.Upsert(nameKey, zoneData)
					if (err != nil) {
						panic(err.Error())
					}
				}
				//NXDOMAIN error, do nothing
			} 
			if (err != nil) {
				fmt.Println("err: " + err.Error())
			}			
			if (resp == nil) {
				fmt.Println("Response is nil?")
			}
		}
		// check for errors
		if err = scanner.Err(); err != nil {
			fmt.Println("Error in reading")
		}
		
		session.Close()
	} else {
		fmt.Println("Error in opening file")
	}

}
