package agent

import (
	"crypto/x509"
	jsoniter "github.com/json-iterator/go"
	"os"
	"os/user"
	"runtime"
	"strings"
	"time"

	"fmt"
	"log"

	"github.com/google/gousb"
	"github.com/miekg/pkcs11"
	amqp "github.com/rabbitmq/amqp091-go"
)

var connectedCards map[string]Card
var newCards map[string]Card
var certificates map[string][]byte
var anyTokenFound bool
var channel *amqp.Channel
var queue amqp.Queue
var config *Config

const (
	brokerAddress = "localhost:9092"
)

func errorHandler(err error, msg string) {
	if err != nil {
		log.Fatalf("%s: %s", msg, err)
	}
}

func decodeAlgorithm(code x509.PublicKeyAlgorithm) string {
	var s string
	switch code {
	case x509.RSA:
		s = "RSA"
	case x509.DSA:
		s = "DSA"
	case x509.ECDSA:
		s = "ECDSA"
	default:
		s = "oops"
	}
	return s
}

func getCardAndCertInfo(pkcs11Lib string) {
	log.Println("Initialize: " + pkcs11Lib)
	p := pkcs11.New(pkcs11Lib)
	err := p.Initialize()
	if err != nil {
		panic(err)
	}

	defer p.Destroy()
	defer p.Finalize()

	slots, err := p.GetSlotList(true)
	if err != nil {
		panic(err)
	}

	for _, slot := range slots {
		info, err := p.GetTokenInfo(slot)
		if err != nil {
			panic(err)
		}
		fmt.Println(info.Label, " --- ", info.ManufacturerID, " --- ", info.Model, " --- ", info.SerialNumber)
		session, err := p.OpenSession(slots[0], pkcs11.CKF_SERIAL_SESSION)
		if err != nil {
			panic(err)
		}
		defer p.CloseSession(session)

		certAttr := []*pkcs11.Attribute{pkcs11.NewAttribute(pkcs11.CKA_CLASS, pkcs11.CKO_CERTIFICATE)}
		err = p.FindObjectsInit(session, certAttr)
		if err != nil {
			panic(err)
		}

		objects, _, err := p.FindObjects(session, 10000)
		if err != nil {
			panic(err)
		}

		for _, obj := range objects {
			attrib := []*pkcs11.Attribute{pkcs11.NewAttribute(pkcs11.CKA_VALUE, nil)}
			attr, err := p.GetAttributeValue(session, obj, attrib)
			if err != nil {
				panic(err)
			}
			cert, err := x509.ParseCertificate([]byte(attr[0].Value))
			if err != nil {
				panic(err)
			}
			fmt.Println(cert.Subject)
			fmt.Println(cert.Issuer)
			fmt.Println(decodeAlgorithm(cert.PublicKeyAlgorithm))
		}
		p.FindObjectsFinal(session)
	}
}

func getCertificate(p *pkcs11.Ctx, slot uint, sn string) {
	log.Printf("Slot: %v, SN: %v", slot, sn)
	session, err := p.OpenSession(slot, pkcs11.CKF_SERIAL_SESSION)
	errorHandler(err, "Could not open session,: %v")
	defer p.CloseSession(session)

	certAttr := []*pkcs11.Attribute{pkcs11.NewAttribute(pkcs11.CKA_CLASS, pkcs11.CKO_CERTIFICATE)}
	err = p.FindObjectsInit(session, certAttr)
	errorHandler(err, "Could init to find objects: %v")

	objects, _, err := p.FindObjects(session, 10000)
	if err != nil {
		panic(err)
	}

	for _, obj := range objects {
		attrib := []*pkcs11.Attribute{pkcs11.NewAttribute(pkcs11.CKA_VALUE, nil)}
		attr, err := p.GetAttributeValue(session, obj, attrib)
		fmt.Println(attr)
		if err != nil {
			panic(err)
		}
		cert, err := x509.ParseCertificate([]byte(attr[0].Value))
		if err != nil {
			panic(err)
		}
		certificateFound(cert.SerialNumber.String(), []byte(attr[0].Value))
	}
	p.FindObjectsFinal(session)
}

// TODO: Check the necessity of `product`
func isCard(vendor string, product string) bool {
	for _, vendorToFind := range config.PossibleVendors {
		if vendor == vendorToFind {
			return true
		}
	}
	return false
}

func getPKCS11Driver(vendor string) (string, error) {
	driver := config.TokenDriver[runtime.GOOS][vendor]
	if driver != "" {
		return driver, nil
	}
	return "Unknown device " + vendor, fmt.Errorf("Unknown device")
}

func cardConnnected(devId string, info pkcs11.TokenInfo) {
	body, err := jsoniter.Marshal(info)
	headers := make(amqp.Table)
	headers["Event"] = "CardConnect"
	headers["ID"] = devId
	host, err := os.Hostname()

	errorHandler(err, "Can't get hostname")
	headers["Hostname"] = host
	user, err := user.Current()
	errorHandler(err, "Can't get username")
	headers["Username"] = user.Username
	err = channel.Publish(
		"",         // exchange
		queue.Name, // routing key
		false,      // mandatory
		false,      // immediate
		amqp.Publishing{
			Timestamp:   time.Now(),
			ContentType: "text/plain",
			Body:        []byte(body),
			Headers:     headers,
		})
	errorHandler(err, "Failed to publish CARD CONNECTED message")
}

func certificateFound(certId string, certBody []byte) {
	headers := make(amqp.Table)
	headers["Event"] = "CertFound"
	headers["ID"] = certId
	host, err := os.Hostname()
	errorHandler(err, "Can't get hostname")
	headers["Hostname"] = host
	user, err := user.Current()
	errorHandler(err, "Can't get username")
	headers["Username"] = user.Username
	err = channel.Publish(
		"",         // exchange
		queue.Name, // routing key
		false,      // mandatory
		false,      // immediate
		amqp.Publishing{
			Timestamp:   time.Now(),
			ContentType: "text/plain",
			Body:        certBody,
			Headers:     headers,
		})
	errorHandler(err, "Failed to publish CERTIFICATE FOUND message")

}

func cardRemoved(devId string, info pkcs11.TokenInfo) {
	body, err := jsoniter.Marshal(info)
	headers := make(amqp.Table)

	headers["Event"] = "CardDisconnect"
	headers["ID"] = devId
	host, err := os.Hostname()

	errorHandler(err, "Can't get hostname")
	headers["Hostname"] = host
	user, err := user.Current()
	errorHandler(err, "Can't get username")
	headers["Username"] = user.Username
	err = channel.Publish(
		"",         // exchange
		queue.Name, // routing key
		false,      // mandatory
		false,      // immediate
		amqp.Publishing{
			Timestamp:   time.Now(),
			ContentType: "text/plain",
			Body:        []byte(body),
			Headers:     headers,
		})
	errorHandler(err, "Failed to publish CARD DISCONNECTED message")

	log.Println("Card removed: ", devId, info)
}

func devFunc(desc *gousb.DeviceDesc) bool {
	// Convert vendor and product codes
	vendor := strings.ToUpper(desc.Vendor.String())
	product := strings.ToUpper(desc.Product.String())
	if isCard(vendor, product) {
		anyTokenFound = true
		driver, err := getPKCS11Driver(vendor)
		if err != nil {
			log.Fatalf("Could not open find driver: %v", err)
		}

		// Init PKCS11
		p := pkcs11.New(driver)
		err = p.Initialize()
		if err != nil {
			log.Fatalf("Could not open init PKCS: %v (driver: %v)", err, driver)
		}

		// Close PKCS11 connection
		defer p.Destroy()
		defer p.Finalize()

		// Get slots with cards / tokens
		slots, err := p.GetSlotList(true)
		if err != nil {
			log.Fatalf("Could not open PKCS11 slots: %v", err)
		}

		// Iterate all slots
		for _, slot := range slots {
			// Get info about card / token
			info, err := p.GetTokenInfo(slot)
			if err != nil {
				log.Fatalf("Could not get card / token info: %v", err)
			} else {
				newCards[info.SerialNumber] = Card{Product: product, Vendor: vendor, Info: info}
				if (connectedCards[info.SerialNumber] == Card{}) {
					getCertificate(p, slot, info.SerialNumber)
				}
			}
		}
		for key, card := range newCards {
			if (connectedCards[key] == Card{}) {
				cardConnnected(key, newCards[key].Info)
				connectedCards[key] = card
			}
		}
		for key := range connectedCards {
			if (newCards[key] == Card{}) {
				cardRemoved(key, connectedCards[key].Info)
				delete(connectedCards, key)
			}
		}
		newCards = make(map[string]Card)
	}
	return false
}

func configHandler(queue amqp.Queue) <-chan amqp.Delivery {
	msgs, err := channel.Consume(
		queue.Name, // queue
		"",         // consumer
		true,       // auto-ack
		false,      // exclusive
		false,      // no-local
		false,      // no-wait
		nil,        // args
	)
	errorHandler(err, "Failed to register a consumer")
	return msgs

}

func RunAgent() {
	// Init RabbitMQ connection
	conn, err := amqp.Dial("amqp://guest:guest@localhost:5672/")
	errorHandler(err, "Failed to connect to RabbitMQ")
	defer conn.Close()

	channel, err = conn.Channel()
	errorHandler(err, "Failed to open a channel")
	defer channel.Close()

	queue, err = channel.QueueDeclare(
		"agent_event", // name
		false,         // durable
		true,          // delete when unused
		false,         // exclusive
		false,         // no-wait
		nil,           // arguments
	)
	errorHandler(err, "Failed to declare a queue")

	// Create new instance of config
	config = new(Config)
	config.Update()
	// Create channel to receive config updates
	configChannel := configHandler(queue)

	certificates = make(map[string][]byte)
	// Create new context
	ctx := gousb.NewContext()
	defer ctx.Close()

	connectedCards = make(map[string]Card)
	newCards = make(map[string]Card)

	ticker := time.NewTicker(time.Second)
	quit := make(chan struct{})
	for {
		select {
		case <-ticker.C:
			anyTokenFound = false
			ctx.OpenDevices(devFunc)
			if !anyTokenFound {
				for key, card := range connectedCards {
					cardRemoved(key, card.Info)
				}
				connectedCards = make(map[string]Card)
			}
		case d := <-configChannel:
			if d.ContentType == "application/json" && d.Type == "config" {
				if err = jsoniter.Unmarshal(d.Body, config); err == nil {
					log.Printf("%s: \n%s", "Received config", d.Body)
					config.Update()
				}
				errorHandler(err, "Failed to unmarshal config")
			}
		case <-quit:
			ticker.Stop()
			return
		}
	}
}
