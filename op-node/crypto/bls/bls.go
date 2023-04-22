package bls

import (
	"crypto/rand"
	"fmt"
	"log"
	"math/big"
	"os"

	"github.com/consensys/gnark-crypto/ecc/bn254"
	"github.com/consensys/gnark-crypto/ecc/bn254/fp"
	"github.com/consensys/gnark-crypto/ecc/bn254/fr"
	"golang.org/x/crypto/sha3"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/crypto"
)

type BlsKeyPair struct {
	private *fr.Element
	public  *bn254.G2Affine
}

const (
	g2Gen_x1 = "10857046999023057135944570762232829481370756359578518086990519993285655852781"
	g2Gen_x2 = "11559732032986387107991004021392285783925812861821192530917403151452391805634"

	g2Gen_y1 = "8495653923123431417604973247489272438418190587263600148770280649306958101930"
	g2Gen_y2 = "4082367875863433681332203403145435568316851327593401208105741076214120093531"
)

func VerifyBlsSig(sig *bn254.G1Affine, pubkey *bn254.G2Affine, msgBytes []byte) bool {
	var g2Gen bn254.G2Affine
	g2Gen.X.SetString(g2Gen_x1, g2Gen_x2)
	g2Gen.Y.SetString(g2Gen_y1, g2Gen_y2)

	msgPoint := MapToCurve(msgBytes)

	var negSig bn254.G1Affine
	negSig.Neg((*bn254.G1Affine)(sig))

	P := [2]bn254.G1Affine{*msgPoint, negSig}
	Q := [2]bn254.G2Affine{*pubkey, g2Gen}

	ok, err := bn254.PairingCheck(P[:], Q[:])
	if err != nil {
		fmt.Println("[Bls] Unable to do pairing check.", err)
		return false
	}
	return ok

}

func GetG2Generator() *bn254.G2Affine {
	g2Gen := new(bn254.G2Affine)
	g2Gen.X.SetString(g2Gen_x1, g2Gen_x2)
	g2Gen.Y.SetString(g2Gen_y1, g2Gen_y2)
	return g2Gen
}

func (k *BlsKeyPair) SignMessage(headerHash []byte) *bn254.G1Affine {
	if len(headerHash) != 32 {
		fmt.Println("SignMessage only on header hash")
		os.Exit(1)
	}

	H := MapToCurve(headerHash)
	sig := new(bn254.G1Affine).ScalarMultiplication(H, k.private.ToBigIntRegular(new(big.Int)))
	return sig
}

func (k *BlsKeyPair) GetPubKey() [64]byte {
	return k.public.Bytes()
}

func (k *BlsKeyPair) GetPubKeyPoint() *bn254.G2Affine {
	return k.public
}

func BlsKeysFromPk(sk *fr.Element) (*BlsKeyPair, error) {

	var g2Gen bn254.G2Affine
	g2Gen.X.SetString(g2Gen_x1, g2Gen_x2)
	g2Gen.Y.SetString(g2Gen_y1, g2Gen_y2)

	pk := new(bn254.G2Affine).ScalarMultiplication(&g2Gen, sk.ToBigIntRegular(new(big.Int)))
	return &BlsKeyPair{sk, pk}, nil
}

func BlsKeysFromString(sk string) (*BlsKeyPair, error) {

	log.Println(sk)

	_sk, err := new(fr.Element).SetString(sk)
	if err != nil {
		return nil, err
	}
	return BlsKeysFromPk(_sk)
}

func GenRandomBlsKeys() (*BlsKeyPair, error) {

	//Max random value, a 381-bits integer, i.e 2^381 - 1
	max := new(big.Int)
	max.SetString("21888242871839275222246405745257275088548364400416034343698204186575808495617", 10)

	//Generate cryptographically strong pseudo-random between 0 - max
	n, err := rand.Int(rand.Reader, max)
	if err != nil {
		return nil, err
	}

	sk := new(fr.Element).SetBigInt(n)
	return BlsKeysFromPk(sk)
}

func HashToCurve(data []byte) *bn254.G1Affine {
	digest := crypto.Keccak256(data)
	return MapToCurve(digest[:])
}

func MapToCurve(digest []byte) *bn254.G1Affine {
	if len(digest) != 32 {
		fmt.Println("only map 32 bytes")
		os.Exit(1)
	}

	//fmt.Println("fp.Modulus", fp.Modulus())

	one := new(big.Int).SetUint64(1)
	three := new(big.Int).SetUint64(3)
	x := new(big.Int)
	x.SetBytes(digest[:])
	for true {
		// y = x^3 + 3
		xP3 := new(big.Int).Exp(x, big.NewInt(3), fp.Modulus())
		y := new(big.Int).Add(xP3, three)
		y.Mod(y, fp.Modulus())
		//fmt.Println("x", x)
		//fmt.Println("y", y)

		if y.ModSqrt(y, fp.Modulus()) == nil {
			x.Add(x, one).Mod(x, fp.Modulus())
		} else {
			var fpX, fpY fp.Element
			fpX.SetBigInt(x)
			fpY.SetBigInt(y)
			return &bn254.G1Affine{
				X: fpX,
				Y: fpY,
			}
		}
	}
	return new(bn254.G1Affine)
}

func BuildKeyFromRegistrationData(data [4]*big.Int) *bn254.G2Affine {

	public := new(bn254.G2Affine)

	public.X.A0.SetBigInt(data[0])
	public.X.A1.SetBigInt(data[1])
	public.Y.A0.SetBigInt(data[2])
	public.Y.A1.SetBigInt(data[3])

	return public
}

// ToDo handle errro
func ConvertStringsToPubKey(data [4]string) *bn254.G2Affine {

	public := new(bn254.G2Affine)

	_, err := public.X.A0.SetString(data[0])
	public.X.A1.SetString(data[1])
	public.Y.A0.SetString(data[2])
	public.Y.A1.SetString(data[3])
	_ = err

	return public
}

func GetPubkeyHash(p *bn254.G2Affine) [32]byte {
	data := SerializeG2(p)

	var pHash [32]byte
	hasher := sha3.NewLegacyKeccak256()
	hasher.Write(data)
	copy(pHash[:], hasher.Sum(nil)[:32])
	return pHash
}

func SerializeG1(p *bn254.G1Affine) []byte {
	calldata := make([]byte, 0)
	tmp := p.X.Bytes()
	for i := 0; i < 32; i++ {
		calldata = append(calldata, tmp[i])
	}
	tmp = p.Y.Bytes()
	for i := 0; i < 32; i++ {
		calldata = append(calldata, tmp[i])
	}
	return calldata
}

func DeserializeG1(b []byte) *bn254.G1Affine {
	p := new(bn254.G1Affine)
	p.X.SetBytes(b[0:32])
	p.Y.SetBytes(b[32:64])
	return p
}

func SerializeG2(p *bn254.G2Affine) []byte {
	calldata := make([]byte, 0)
	tmp := p.X.A0.Bytes()
	for i := 0; i < 32; i++ {
		calldata = append(calldata, tmp[i])
	}
	tmp = p.X.A1.Bytes()
	for i := 0; i < 32; i++ {
		calldata = append(calldata, tmp[i])
	}
	tmp = p.Y.A0.Bytes()
	for i := 0; i < 32; i++ {
		calldata = append(calldata, tmp[i])
	}
	tmp = p.Y.A1.Bytes()
	for i := 0; i < 32; i++ {
		calldata = append(calldata, tmp[i])
	}
	return calldata
}

func DeserializeG2(b []byte) *bn254.G2Affine {
	p := new(bn254.G2Affine)
	p.X.A0.SetBytes(b[0:32])
	p.X.A1.SetBytes(b[32:64])
	p.Y.A0.SetBytes(b[64:96])
	p.Y.A1.SetBytes(b[96:128])
	return p
}

func (k *BlsKeyPair) MakePubkeyRegistrationData(operator common.Address) []byte {
	calldata := make([]byte, 0)
	tmp := k.public.X.A1.Bytes()
	for i := 0; i < 32; i++ {
		calldata = append(calldata, tmp[i])
	}
	tmp = k.public.X.A0.Bytes()
	for i := 0; i < 32; i++ {
		calldata = append(calldata, tmp[i])
	}
	tmp = k.public.Y.A1.Bytes()
	for i := 0; i < 32; i++ {
		calldata = append(calldata, tmp[i])
	}
	tmp = k.public.Y.A0.Bytes()
	for i := 0; i < 32; i++ {
		calldata = append(calldata, tmp[i])
	}
	// fmt.Println(hex.EncodeToString(pubkeyBytes))
	// fmt.Println(hashToCurve(pubkeyBytes).String())
	h := HashToCurve(append(calldata, operator.Bytes()...))
	// fmt.Println(h.IsInSubGroup())
	// fmt.Println(h.X.String())
	// fmt.Println(h.Y.String())
	// fmt.Println(pk.X.A0.String())
	// fmt.Println(pk.X.A1.String())
	// fmt.Println(pk.Y.A0.String())
	// fmt.Println(pk.Y.A1.String())
	sigma := *new(bn254.G1Affine).ScalarMultiplication(h, k.private.ToBigIntRegular(new(big.Int)))
	tmp = sigma.X.Bytes()
	for i := 0; i < 32; i++ {
		calldata = append(calldata, tmp[i])
	}
	tmp = sigma.Y.Bytes()
	for i := 0; i < 32; i++ {
		calldata = append(calldata, tmp[i])
	}
	// fmt.Println(sigma.X.String())
	// fmt.Println(sigma.Y.String())
	return calldata
}

func printG2Quotes(point *bn254.G2Jac) {
	fmt.Println()
	fmt.Println()
	fmt.Printf("[\"%s\",\"%s\",\"%s\",\"%s\",\"%s\",\"%s\"]", point.X.A0.String(), point.X.A1.String(), point.Y.A0.String(), point.Y.A1.String(), point.Z.A0.String(), point.Z.A1.String())
}
func printG2(point *bn254.G2Jac) {
	fmt.Println()
}

func ConvertFrameBlsKzgToBytes(castedG1Point *bn254.G1Affine) [64]byte {
	castedG1LowDegreeProof := *(castedG1Point)
	lowDegreeProof := SerializeG1(&castedG1LowDegreeProof)
	var lowDegreeProof64 [64]byte
	copy(lowDegreeProof64[:], lowDegreeProof[:])
	return lowDegreeProof64
}
