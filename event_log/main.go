package main

import (
	"crypto/x509"
	"encoding/hex"
	"encoding/pem"
	"flag"
	"fmt"
	"log"
	"os"

	"github.com/golang/glog"
	"github.com/google/go-tpm-tools/client"
	pb "github.com/google/go-tpm-tools/proto/tpm"
	"github.com/google/go-tpm-tools/server"
	"github.com/google/go-tpm/legacy/tpm2"
)

const defaultRSAExponent = 1<<16 + 1

var handleNames = map[string][]tpm2.HandleType{
	"all":       []tpm2.HandleType{tpm2.HandleTypeLoadedSession, tpm2.HandleTypeSavedSession, tpm2.HandleTypeTransient},
	"loaded":    []tpm2.HandleType{tpm2.HandleTypeLoadedSession},
	"saved":     []tpm2.HandleType{tpm2.HandleTypeSavedSession},
	"transient": []tpm2.HandleType{tpm2.HandleTypeTransient},
}

var (
	tpmPath           = flag.String("tpm-path", "/dev/tpm0", "Path to the TPM device (character device or a Unix socket).")
	pcr               = flag.Int("pcr", 0, "PCR to seal data to. Must be within [0, 23].")
	pcrValue          = flag.String("pcrValue", "0f2d3a2a1adaa479aeeca8f5df76aadc41b862ea", "PCR value. on GCP Shielded VM, debian10 with secureboot: 0f2d3a2a1adaa479aeeca8f5df76aadc41b862ea is for PCR 0")
	defaultEKTemplate = tpm2.Public{
		Type:    tpm2.AlgRSA,
		NameAlg: tpm2.AlgSHA256,
		Attributes: tpm2.FlagFixedTPM | tpm2.FlagFixedParent | tpm2.FlagSensitiveDataOrigin |
			tpm2.FlagAdminWithPolicy | tpm2.FlagRestricted | tpm2.FlagDecrypt,
		AuthPolicy: []byte{
			0x83, 0x71, 0x97, 0x67, 0x44, 0x84,
			0xB3, 0xF8, 0x1A, 0x90, 0xCC, 0x8D,
			0x46, 0xA5, 0xD7, 0x24, 0xFD, 0x52,
			0xD7, 0x6E, 0x06, 0x52, 0x0B, 0x64,
			0xF2, 0xA1, 0xDA, 0x1B, 0x33, 0x14,
			0x69, 0xAA,
		},
		RSAParameters: &tpm2.RSAParams{
			Symmetric: &tpm2.SymScheme{
				Alg:     tpm2.AlgAES,
				KeyBits: 128,
				Mode:    tpm2.AlgCFB,
			},
			KeyBits:    2048,
			ModulusRaw: make([]byte, 256),
		},
	}

	defaultKeyParams = tpm2.Public{
		Type:    tpm2.AlgRSA,
		NameAlg: tpm2.AlgSHA256,
		Attributes: tpm2.FlagFixedTPM | tpm2.FlagFixedParent | tpm2.FlagSensitiveDataOrigin |
			tpm2.FlagUserWithAuth | tpm2.FlagRestricted | tpm2.FlagSign,
		AuthPolicy: []byte{},
		RSAParameters: &tpm2.RSAParams{
			Sign: &tpm2.SigScheme{
				Alg:  tpm2.AlgRSASSA,
				Hash: tpm2.AlgSHA256,
			},
			KeyBits: 2048,
		},
	}
)

func main() {
	flag.Parse()

	if *pcr < 0 || *pcr > 23 {
		fmt.Fprintf(os.Stderr, "Invalid flag 'pcr': value %d is out of range", *pcr)
		os.Exit(1)
	}

	var err error

	glog.V(2).Infof("======= Init CreateKeys ========")

	rwc, err := tpm2.OpenTPM(*tpmPath)
	if err != nil {
		glog.Fatalf("can't open TPM %q: %v", tpmPath, err)
	}
	defer func() {
		if err := rwc.Close(); err != nil {
			glog.Fatalf("%v\ncan't close TPM: %v", tpmPath, err)
		}
	}()

	totalHandles := 0
	for _, handleType := range handleNames["all"] {
		handles, err := client.Handles(rwc, handleType)
		if err != nil {
			glog.Fatalf("getting handles: %v", err)
		}
		for _, handle := range handles {
			if err = tpm2.FlushContext(rwc, handle); err != nil {
				glog.Fatalf("flushing handle 0x%x: %v", handle, err)
			}
			glog.V(2).Infof("Handle 0x%x flushed\n", handle)
			totalHandles++
		}
	}

	glog.V(2).Infof("%d handles flushed\n", totalHandles)

	pcrList := []int{*pcr}
	pcrval, err := tpm2.ReadPCR(rwc, *pcr, tpm2.AlgSHA1)
	if err != nil {
		glog.Fatalf("Unable to  ReadPCR : %v", err)
	}
	glog.V(2).Infof("PCR %v Value %v ", pcr, hex.EncodeToString(pcrval))

	pcrSelection23 := tpm2.PCRSelection{Hash: tpm2.AlgSHA1, PCRs: pcrList}
	emptyPassword := ""

	glog.V(2).Infof("======= createPrimary ========")

	ekh, _, err := tpm2.CreatePrimary(rwc, tpm2.HandleEndorsement, pcrSelection23, emptyPassword, emptyPassword, defaultEKTemplate)
	if err != nil {
		glog.Fatalf("creating EK: %v", err)
	}
	defer tpm2.FlushContext(rwc, ekh)

	// reread the pub eventhough tpm2.CreatePrimary* gives pub
	tpmEkPub, name, _, err := tpm2.ReadPublic(rwc, ekh)
	if err != nil {
		glog.Fatalf("ReadPublic failed: %s", err)
	}

	p, err := tpmEkPub.Key()
	if err != nil {
		glog.Fatalf("tpmEkPub.Key() failed: %s", err)
	}
	glog.V(10).Infof("tpmEkPub: \n%v", p)

	b, err := x509.MarshalPKIXPublicKey(p)
	if err != nil {
		glog.Fatalf("Unable to convert ekpub: %v", err)
	}

	ekPubPEM := pem.EncodeToMemory(
		&pem.Block{
			Type:  "PUBLIC KEY",
			Bytes: b,
		},
	)
	glog.V(2).Infof("ekPub Name: %v", hex.EncodeToString(name))
	glog.V(2).Infof("ekPub: \n%v", string(ekPubPEM))

	glog.V(2).Infof("======= CreateKeyUsingAuth ========")

	sessCreateHandle, _, err := tpm2.StartAuthSession(
		rwc,
		tpm2.HandleNull,
		tpm2.HandleNull,
		make([]byte, 16),
		nil,
		tpm2.SessionPolicy,
		tpm2.AlgNull,
		tpm2.AlgSHA256)
	if err != nil {
		glog.Fatalf("Unable to create StartAuthSession : %v", err)
	}
	defer tpm2.FlushContext(rwc, sessCreateHandle)

	if _, err := tpm2.PolicySecret(rwc, tpm2.HandleEndorsement, tpm2.AuthCommand{Session: tpm2.HandlePasswordSession, Attributes: tpm2.AttrContinueSession}, sessCreateHandle, nil, nil, nil, 0); err != nil {
		glog.Fatalf("Unable to create PolicySecret: %v", err)
	}

	authCommandCreateAuth := tpm2.AuthCommand{Session: sessCreateHandle, Attributes: tpm2.AttrContinueSession}

	akPriv, akPub, creationData, creationHash, creationTicket, err := tpm2.CreateKeyUsingAuth(rwc, ekh, pcrSelection23, authCommandCreateAuth, emptyPassword, defaultKeyParams)
	if err != nil {
		glog.Fatalf("CreateKey failed: %s", err)
	}
	glog.V(5).Infof("akPub: %v,", hex.EncodeToString(akPub))
	glog.V(5).Infof("akPriv: %v,", hex.EncodeToString(akPriv))

	cr, err := tpm2.DecodeCreationData(creationData)
	if err != nil {
		glog.Fatalf("Unable to  DecodeCreationData : %v", err)
	}

	glog.V(10).Infof("CredentialData.ParentName.Digest.Value %v", hex.EncodeToString(cr.ParentName.Digest.Value))
	glog.V(10).Infof("CredentialTicket %v", hex.EncodeToString(creationTicket.Digest))
	glog.V(10).Infof("CredentialHash %v", hex.EncodeToString(creationHash))

	defer tpm2.FlushContext(rwc, ekh)

	glog.V(2).Infof("======= LoadUsingAuth ========")

	loadCreateHandle, _, err := tpm2.StartAuthSession(
		rwc,
		tpm2.HandleNull,
		tpm2.HandleNull,
		make([]byte, 16),
		nil,
		tpm2.SessionPolicy,
		tpm2.AlgNull,
		tpm2.AlgSHA256)
	if err != nil {
		glog.Fatalf("Unable to create StartAuthSession : %v", err)
	}
	defer tpm2.FlushContext(rwc, loadCreateHandle)

	if _, err := tpm2.PolicySecret(rwc, tpm2.HandleEndorsement, tpm2.AuthCommand{Session: tpm2.HandlePasswordSession, Attributes: tpm2.AttrContinueSession}, loadCreateHandle, nil, nil, nil, 0); err != nil {
		glog.Fatalf("Unable to create PolicySecret: %v", err)
	}

	authCommandLoad := tpm2.AuthCommand{Session: loadCreateHandle, Attributes: tpm2.AttrContinueSession}

	keyHandle, keyName, err := tpm2.LoadUsingAuth(rwc, ekh, authCommandLoad, akPub, akPriv)
	if err != nil {
		log.Fatalf("Load failed: %s", err)
	}
	defer tpm2.FlushContext(rwc, keyHandle)
	kn := hex.EncodeToString(keyName)
	glog.V(2).Infof("ak keyName %v", kn)

	evtLog, err := client.GetEventLog(rwc)
	if err != nil {
		glog.Fatalf("failed to get event log: %v", err)
	}

	bt, err := hex.DecodeString(*pcrValue)
	if err != nil {
		glog.Fatalf("Error decoding pcr %v", err)
	}
	pcrMap := map[uint32][]byte{uint32(*pcr): bt}

	pcrs := &pb.PCRs{Hash: pb.HashAlgo_SHA1, Pcrs: pcrMap}

	events, err := server.ParseAndVerifyEventLog(evtLog, pcrs)
	if err != nil {
		glog.Fatalf("failed to read PCRs: %v", err)
	}

	for _, event := range events {
		glog.V(2).Infof("Event Type %v\n", event.Type)
		glog.V(2).Infof("PCR Index %d\n", event.Index)
		glog.V(2).Infof("Event Data %s\n", hex.EncodeToString(event.Data))
		glog.V(2).Infof("Event Digest %s\n", hex.EncodeToString(event.Digest))
	}
	glog.V(2).Infof("EventLog Verified ")

}
