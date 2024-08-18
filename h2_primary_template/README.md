Generate a primary using the specified format described in [ASN.1 Specification for TPM 2.0 Key Files](https://www.hansenpartnership.com/draft-bottomley-tpm2-keys.html#name-parent)  where the template h-2 is described in pg 43 [TCG EK Credential Profile](https://trustedcomputinggroup.org/wp-content/uploads/TCG_IWG_EKCredentialProfile_v2p4_r2_10feb2021.pdf)

the priamry is formatted for use with openssl


```bash
## create the primary

printf '\x00\x00' > unique.dat
tpm2_createprimary -C o -G ecc  -g sha256  -c primary.ctx -a "fixedtpm|fixedparent|sensitivedataorigin|userwithauth|noda|restricted|decrypt" -u unique.dat

# create parent
tpm2_create -G rsa2048:rsassa:null -g sha256 -u key.pub -r key.priv -C primary.ctx
tpm2_load -C primary.ctx -u key.pub -r key.priv -c key.ctx 

## sign some data
echo -n "foo" > data.txt 
tpm2_sign -c key.ctx -g sha256 -o sig.rssa -f plain data.txt

xxd -p sig.rssa 
    16a4a291d764c318f38ff647f9438ce2f57a49b34a11a608daaf2c6c3341
    6f657ff025dca825b01ec6b3347984e4ba86760f3e4b987f84bbcd43c1e2
    c68ba83fcf5bb5ef9e8e379204ac0ccdfd5ed26281bab5b6d12e9e460929
    284a5b1632822e4987833679663538208d83b20d0783b7a2d812c369e271
    0209ffed3b6456b295950eeee1c33f2a0cea2a31c9a05184b461fe70f7bd

## generate the PEM representation
tpm2tss-genkey -u key.pub -r key.priv private.pem

$ cat private.pem 
-----BEGIN TSS2 PRIVATE KEY-----
MIICFAYGZ4EFCgEDoAMBAf8CBEAAAAEEggEaARgAAQALAAQAcgAAABAAFAALCAAA
AAAAAQDNmPPx25bcaW/9iWROnkG6GRDk4pZ3ijdhAReacawCEEWfeVQ/3P/FBnl5
bzv0eAZBoyVcAwn+mPyBtTseiLT7Kwr9K/ycy2OccdgVXBEzC7fVHkJ383BO+1eB
BIVnuK5LrnAPUzvqZmT4DrTZSqMvBB1o3YwrMOs7BV7TIwNL7oh/7mXby6J3oIJu
iN012zu5/LeT51rTjcgzx9iSLYiSLCGRyYnGuvQvnzthOVhMaUoAxe9QgsIlVRip
+uDuJpdOUgJZC94exzhnfx5hc4d4csxn+/W4kXpdZ9ix0CEEu4WGku4h1RbS0mqe
fNNMkpIRK1nBgB7j/uRIN3N0gfUtBIHgAN4AINr/5le1aGDuMwzMiuhK4SQrQ5Ww
Bk6oQg7ehZnC545UABBwx6xkUVFRLh7rbVQrw2CDtPDeXY08T4VWWpFbtJzsGrWU
qiXBMVrlAwyCMb1hS126A9s92fAwCR/1nojsbYH/cs3EbJsdYuOo7vMjB5SMkOaB
MxvI96A5WJSmqRgOKbesbn2eRElUTAgL9aLUkJ+AdrVdCCocg6WT2GwHwBckI8Y8
rYp+bAhb97JurBfcp7u4ZRC5nWv5S51BWZ1Kp+lLcFGx+I78EEugZYrijiUqrTeh
EBqFFpOTT9Y=
-----END TSS2 PRIVATE KEY-----


$ openssl asn1parse -inform PEM -in private.pem 
    0:d=0  hl=4 l= 532 cons: SEQUENCE          
    4:d=1  hl=2 l=   6 prim: OBJECT            :2.23.133.10.1.3
   12:d=1  hl=2 l=   3 cons: cont [ 0 ]        
   14:d=2  hl=2 l=   1 prim: BOOLEAN           :255
   17:d=1  hl=2 l=   4 prim: INTEGER           :40000001
   23:d=1  hl=4 l= 282 prim: OCTET STRING      [HEX DUMP]:01180001000B00040072000000100014000B0800000000000100CD98F3F1DB96DC696FFD89644E9E41BA1910E4E296778A376101179A71AC0210459F79543FDCFFC50679796F3BF4780641A3255C0309FE98FC81B53B1E88B4FB2B0AFD2BFC9CCB639C71D8155C11330BB7D51E4277F3704EFB5781048567B8AE4BAE700F533BEA6664F80EB4D94AA32F041D68DD8C2B30EB3B055ED323034BEE887FEE65DBCBA277A0826E88DD35DB3BB9FCB793E75AD38DC833C7D8922D88922C2191C989C6BAF42F9F3B6139584C694A00C5EF5082C2255518A9FAE0EE26974E5202590BDE1EC738677F1E6173877872CC67FBF5B8917A5D67D8B1D02104BB858692EE21D516D2D26A9E7CD34C9292112B59C1801EE3FEE44837737481F52D
  309:d=1  hl=3 l= 224 prim: OCTET STRING      [HEX DUMP]:00DE0020DAFFE657B56860EE330CCC8AE84AE1242B4395B0064EA8420EDE8599C2E78E54001070C7AC645151512E1EEB6D542BC36083B4F0DE5D8D3C4F85565A915BB49CEC1AB594AA25C1315AE5030C8231BD614B5DBA03DB3DD9F030091FF59E88EC6D81FF72CDC46C9B1D62E3A8EEF32307948C90E681331BC8F7A0395894A6A9180E29B7AC6E7D9E4449544C080BF5A2D4909F8076B55D082A1C83A593D86C07C0172423C63CAD8A7E6C085BF7B26EAC17DCA7BBB86510B99D6BF94B9D41599D4AA7E94B7051B1F88EFC104BA0658AE28E252AAD37A1101A851693934FD6
```


Now  load the key using go-tpm and keyfile

```bash


$ go run main.go 

2024/05/31 22:06:27 ======= Init  ========
2024/05/31 22:06:27 signature from go-tpm-tools key : 16a4a291d764c318f38ff647f9438ce2f57a49b34a11a608daaf2c6c33416f657ff025dca825b01ec6b3347984e4ba86760f3e4b987f84bbcd43c1e2c68ba83fcf5bb5ef9e8e379204ac0ccdfd5ed26281bab5b6d12e9e460929284a5b1632822e4987833679663538208d83b20d0783b7a2d812c369e2710209ffed3b6456b295950eeee1c33f2a0cea2a31c9a05184b461fe70f7bd0b333c3d78ff22844c1f9b9543259eec5c6183b3019fcc23ca5002097dcafe38a620023fca79499af7ce15b86c87d644ddc2b3313e52cd2a5b711d7836cfc39f7417e61414d019d5a300e4fcf8ccd9399ddc3137a4c5dd379065e573a1fc0478a8bc407db49f60092613
```