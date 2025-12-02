package awsEncryptionMaterials

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"math/big"

	hsmClient "awsClient/pkg/requestHSMclient"

	"github.com/aws/amazon-s3-encryption-client-go/v3/materials"
)

// Ce fichier défini le Cryptographic Material Manager pour le AWS S3 encryption client

const (
	AESCBCPKCS5Padding = "AES/CBC/PKCS5Padding"
	// aesCbcTagSizeBits  = "0"
	defaultAlgorithm = "AES/GCM/NoPadding"
	GcmTagSizeBits   = "128"
	// gcmKeySize         = 32
	gcmNonceSize      = 12
	EncryptionContext = "EncryptionContext"
)

type CustomCryptographicMaterialsManager struct {
	// pour récuperer le matériel crytographique, le crypto material manager a besoin de :
	// - l'adresse du client HSM, qui est capable de récupérer les clés sur le HSM
	hsm_client_address string
	// - les infos le la clé qu'il va utiliser
	// on lui passe deux structures hsmClient.KeyHSM
	// qui donnent le couple (numéro du keystore, index de la clé)
	// ce qui permet de référencer deux endroits où se situe la clé
	keyHSM_1 hsmClient.KeyHSM
	keyHSM_2 hsmClient.KeyHSM
}

type FavContextKey string

// crée un cryptographic material manager qui s'occupe de gérer le matériel de chiffrement
// pour le S3 encryption client.
// on lui passe l'adresse du client HSM pour faire des requêtes de clés,
// et deux addresse de la clé qu'on veut sur des keystores différents
func NewCustomCryptographicMaterialsManager(hsm_client_address string, keyHSM_1 hsmClient.KeyHSM, keyHSM_2 hsmClient.KeyHSM) *CustomCryptographicMaterialsManager {
	return &CustomCryptographicMaterialsManager{
		hsm_client_address: hsm_client_address,
		keyHSM_1:           keyHSM_1,
		keyHSM_2:           keyHSM_2,
	}
}

func GenerateBytes(n int) ([]byte, error) {
	b := make([]byte, n)
	_, err := rand.Read(b)
	if err != nil {
		return nil, err
	}
	return b, nil
}

// Fonctions utilisées pour le chiffrement/déchiffrement
func (ccm *CustomCryptographicMaterialsManager) GetEncryptionMaterials(ctx context.Context, matDesc materials.MaterialDescription) (*materials.CryptographicMaterials, error) {
	// ici on envoie une requête au client HSM, qui va récupérer la clé stockée aux emplacements
	// donnés en entrée. Cette fonction fait deux requêtes parallèles et renvoie le résultat
	// de la première requête qui a terminé.
	k := make([]byte, 32)
	_, err := rand.Read(k)
	if err != nil {
		panic(err)
	}

	// fmt.Printf("Un nombre random %x\n", k)
	key := hsmClient.GetKey(ccm.hsm_client_address, ccm.keyHSM_1, ccm.keyHSM_2, "CreateCk", k)
	hexStr := fmt.Sprintf("%x", key)
	// fmt.Println("Key Get from HSM : ", hexStr)
	if len(key) == 0 {
		return nil, fmt.Errorf("couldn't retrieve key for encryption")
	}

	k2 := *big.NewInt(1)
	key2 := k2.Bytes()

	// vecteur d'initialisation
	iv, err := GenerateBytes(gcmNonceSize)
	if err != nil {
		return &materials.CryptographicMaterials{}, err
	}
	newMatDesc := materials.MaterialDescription{
		"ck": hexStr,
	}

	// on crée un cryptographicMaterials avec les infos pour le chiffrement
	cryptoMaterials := &materials.CryptographicMaterials{
		Key:          k, // on lui passe la clé récupérée auprès du client HSM
		IV:           iv,
		CEKAlgorithm: defaultAlgorithm,
		TagLength:    GcmTagSizeBits,
		EncryptedKey: key2,

		MaterialDescription: newMatDesc,
	}
	// on renvoie le cryptographic Material
	return cryptoMaterials, nil
}

func (ccm *CustomCryptographicMaterialsManager) DecryptMaterials(ctx context.Context, req materials.DecryptMaterialsRequest) (*materials.CryptographicMaterials, error) {
	// récupération de la clé en faisant une requête au client HSM
	// fmt.Printf("Contenu complet de la requête de déchiffrement (req) : %+v\n", req)

	md := materials.MaterialDescription{}
	err := md.DecodeDescription([]byte(req.MatDesc))
	if err != nil {
		return nil, fmt.Errorf("failed to decode material description: %w", err)
	}
	ck, ok := md["ck"]
	if !ok {
		return nil, fmt.Errorf("ck not find.")
	}

	// fmt.Println("ck values : ", ck)
	// TODO
	ckbytes, err := hex.DecodeString(ck)
	if err != nil {
		panic(err)
	}

	key := hsmClient.GetKey(ccm.hsm_client_address, ccm.keyHSM_1, ccm.keyHSM_2, "GetKFromCK", ckbytes)
	// hexStr := fmt.Sprintf("%x", key)
	// fmt.Println("Key Get from HSM : ", hexStr)

	if len(key) == 0 {
		return nil, fmt.Errorf("couldn't retrieve key for decryption")
	}
	k2 := *big.NewInt(1)
	key2 := k2.Bytes()
	// on crée un cryptographicMaterials avec les infos pour le déchiffrement
	cryptoMaterials := &materials.CryptographicMaterials{
		Key:          key,
		IV:           req.Iv,
		CEKAlgorithm: defaultAlgorithm,
		TagLength:    GcmTagSizeBits,
		EncryptedKey: key2,
	}
	// on renvoie le cryptographic Material
	return cryptoMaterials, nil
}
