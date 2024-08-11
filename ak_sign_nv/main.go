package main

import (
	"crypto"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/x509"
	"encoding/base64"
	"encoding/hex"
	"encoding/pem"
	"flag"
	"log"

	"github.com/google/go-tpm-tools/client"
	//"github.com/google/go-tpm/tpm2"
	"github.com/google/go-tpm/legacy/tpm2"
	"github.com/google/go-tpm/tpmutil"
)

const (
	//tpmDevice = "/dev/tpmrm0"
	// handles https://github.com/google/go-tpm-tools/blob/master/client/handles.go#L36-L43
	emptyPassword = ""
)

var (
	handleNames = map[string][]tpm2.HandleType{
		"all":       {tpm2.HandleTypeLoadedSession, tpm2.HandleTypeSavedSession, tpm2.HandleTypeTransient},
		"loaded":    {tpm2.HandleTypeLoadedSession},
		"saved":     {tpm2.HandleTypeSavedSession},
		"transient": {tpm2.HandleTypeTransient},
	}

	tpmPath = flag.String("tpm-path", "/dev/tpmrm0", "Path to the TPM device (character device or a Unix socket).")
)

func main() {

	flag.Parse()
	log.Println("======= Init  ========")

	rwc, err := tpm2.OpenTPM(*tpmPath)
	if err != nil {
		log.Fatalf("can't open TPM %v: %v", tpmPath, err)
	}
	defer func() {
		if err := rwc.Close(); err != nil {
			log.Fatalf("can't close TPM %v: %v", tpmPath, err)
		}
	}()

	totalHandles := 0
	for _, handleType := range handleNames["all"] {
		handles, err := client.Handles(rwc, handleType)
		if err != nil {
			log.Fatalf("getting handles: %v", err)
		}
		for _, handle := range handles {
			if err = tpm2.FlushContext(rwc, handle); err != nil {
				log.Fatalf("flushing handle 0x%x: %v", handle, err)
			}
			log.Printf("Handle 0x%x flushed\n", handle)
			totalHandles++
		}
	}

	log.Printf("%d handles flushed\n", totalHandles)

	// *****************

	log.Printf("     Load SigningKey and Certifcate ")
	kk, err := client.EndorsementKeyFromNvIndex(rwc, client.GceAKTemplateNVIndexRSA)
	if err != nil {
		log.Printf("ERROR:  could not get EndorsementKeyFromNvIndex: %v", err)
		return
	}
	pubKey := kk.PublicKey().(*rsa.PublicKey)
	akBytes, err := x509.MarshalPKIXPublicKey(pubKey)
	if err != nil {
		log.Printf("ERROR:  could not get MarshalPKIXPublicKey: %v", err)
		return
	}
	akPubPEM := pem.EncodeToMemory(
		&pem.Block{
			Type:  "PUBLIC KEY",
			Bytes: akBytes,
		},
	)
	log.Printf("     Signing PEM \n%s", string(akPubPEM))

	// begin sign with AK using go-tpm-tools
	aKdataToSign := []byte("foobar")
	r, err := kk.SignData(aKdataToSign)
	if err != nil {
		log.Printf("ERROR:  error singing with go-tpm-tools: %v", err)
		return
	}

	log.Printf("     AK Signed Data using go-tpm-tools %s", base64.StdEncoding.EncodeToString(r))

	h := sha256.New()
	h.Write(aKdataToSign)
	if err := rsa.VerifyPKCS1v15(pubKey, crypto.SHA256, h.Sum(nil), r); err != nil {
		log.Printf("ERROR:  could  VerifyPKCS1v15 (signing): %v", err)
		return
	}
	log.Printf("     Signature Verified")

	// begin sign using go-tpm
	//aKkeyHandle := kk.Handle()

	data, err := tpm2.NVReadEx(rwc, tpmutil.Handle(client.GceAKTemplateNVIndexRSA), tpm2.HandleOwner, "", 0)
	if err != nil {
		log.Fatalf("read error at index %d: %v", client.GceAKTemplateNVIndexRSA, err)
	}
	template, err := tpm2.DecodePublic(data)
	if err != nil {
		log.Fatalf("index %d data was not a TPM key template: %v", client.GceAKTemplateNVIndexRSA, err)
	}

	aKkeyHandle, keyName, _, _, _, _, err := tpm2.CreatePrimaryEx(rwc, tpm2.HandleEndorsement, tpm2.PCRSelection{}, emptyPassword, emptyPassword, template)
	if err != nil {
		log.Fatalf("Load AK failed: %s", err)
	}
	defer tpm2.FlushContext(rwc, aKkeyHandle)

	log.Printf("akPub Name: %v", hex.EncodeToString(keyName))
	// ***********

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
		log.Fatalf("Unable to create StartAuthSession : %v", err)
	}
	defer tpm2.FlushContext(rwc, sessCreateHandle)

	if _, _, err := tpm2.PolicySecret(rwc, tpm2.HandleEndorsement, tpm2.AuthCommand{Session: tpm2.HandlePasswordSession, Attributes: tpm2.AttrContinueSession}, sessCreateHandle, nil, nil, nil, 0); err != nil {
		log.Printf("ERROR:  could  PolicySecret (signing): %v", err)
		return
	}

	aKdigest, aKvalidation, err := tpm2.Hash(rwc, tpm2.AlgSHA256, aKdataToSign, tpm2.HandleOwner)
	if err != nil {
		log.Printf("ERROR:  could  StartAuthSession (signing): %v", err)
		return
	}
	log.Printf("     AK Issued Hash %s", base64.StdEncoding.EncodeToString(aKdigest))
	aKsig, err := tpm2.Sign(rwc, aKkeyHandle, emptyPassword, aKdigest, aKvalidation, &tpm2.SigScheme{
		Alg:  tpm2.AlgRSASSA,
		Hash: tpm2.AlgSHA256,
	})
	if err != nil {
		log.Printf("ERROR:  could  Sign (signing): %v", err)
		return
	}
	log.Printf("     AK Signed Data using go-tpm %s", base64.StdEncoding.EncodeToString(aKsig.RSA.Signature))

	if err := rsa.VerifyPKCS1v15(pubKey, crypto.SHA256, aKdigest, aKsig.RSA.Signature); err != nil {
		log.Printf("ERROR:  could  VerifyPKCS1v15 (signing): %v", err)
		return
	}
	log.Printf("     Signature Verified")
	err = tpm2.FlushContext(rwc, sessCreateHandle)
	if err != nil {
		log.Printf("ERROR:  could  flush SessionHandle: %v", err)
		return
	}
	err = tpm2.FlushContext(rwc, aKkeyHandle)
	if err != nil {
		log.Printf("ERROR:  could  flush aKkeyHandle: %v", err)
		return
	}
	kk.Close()

}
