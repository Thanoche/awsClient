package awsClient

import (
	"bufio"
	"context"
	"fmt"
	"log"
	"os"
	"strings"
	"time"

	MyMaterials "awsClient/pkg/awsEncryptionMaterials"

	"github.com/aws/amazon-s3-encryption-client-go/v3/client"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/feature/s3/manager"
	"github.com/aws/aws-sdk-go-v2/service/s3"
)

// Fichier similaire au fichier Get, le principe des fonctions est identique
func PutObject(client *client.S3EncryptionClientV3, chemin, bucket, key string) (*s3.PutObjectOutput, error) {
	// On mesure le temps que prend l'action Put
	start := time.Now()
	defer func() {
		duration := time.Since(start)
		fmt.Printf("L'action Put a pris %v\n", duration)
	}()
	info, err := os.Stat(chemin)
	if err != nil {
		return nil, err
	}

	// On regarde s'il s'agit d'un repertoire ou d'un simple fichier
	if !info.IsDir() {
		// fmt.Println("avant la lecture du fichier")
		file, err := os.Open(chemin)
		if err != nil {
			log.Fatalf("error lecture fichier: %v", err)
		}
		// En fait le contexte ici est une donnée qui sera transmise au Cryptographic Materials manager (cf mymaterials/cmm.go)
		// Concrètement, on lui donne ici une valeur x, cette valeur n'est pas utile dans le cas de la connexion via le serveur HSM
		// Cependant elle est essentielle pour le protocole TPRF
		// Dans notre cas, il faudrait remplacer la valeur x par l'indice i (entier) de l'emplacement mémoire de la clé sur l'HSM
		// Remarque : Il faut aussi s'assurer que les clés ne changent pas de place sur HSM et qu'elles ne sont pas effacées sinon le fichier est perdue
		ctx := context.TODO()
		ctx = context.WithValue(ctx, MyMaterials.FavContextKey("x"), []byte(bucket+"/"+key))

		// fmt.Println("juste avant le test de la taille")
		// cas d'un gros fichier
		if info.Size() > 500*1000*1000 {
			fmt.Println("gros fichier !")

			// Create an uploader with the client and custom options
			uploader := manager.NewUploader(client, func(u *manager.Uploader) {
				u.PartSize = 10 * 1024 * 1024 // 1MB per part
			})

			// Lancer l'upload avec le chiffrement via EncryptionClient
			result, err := uploader.Upload(ctx, &s3.PutObjectInput{
				Bucket: aws.String(bucket),
				Key:    aws.String(key),
				Body:   file,
			})
			if err != nil {
				fmt.Println("Erreur lors de l'upload multipart :", err)
				return nil, err
			}

			// TODO : gérer le cas de la sortie de l'upload

			fmt.Println("Upload multipart complété avec succès:", result.Location)
			return nil, nil
		} else {
			// cas d'un petit fichier
			// Explication de PUT:
			// On appelle la fonction Put via un client s3 qu'on a créé au début du code et auquel on a préalablement
			// associé un cryptographic material manager (CMM) (cf les fonctions "Connexion_aws_[...]"" du fichier
			// init.go, qui se chargent de créer un client associé au CMM demandé par l'utilisateur).
			// Dans le code du CMM (cf fichier mymaterials/cmm.go), on peut voir la fonction GetEncryptionMaterials()
			// qui explicite l'algorithme de chiffrement et la clé à utiliser.
			// Le champ "Key" qu'on passe ci-dessous n'est pas la clé de chiffrement, c'est le chemin du fichier dans S3
			// (cf traiterPut() ci-dessous) et c'est une valeur qu'on passe à TPRF pour générer une clé de chiffrement
			// fmt.Println("body: ", file)
			out, err := client.PutObject(ctx, &s3.PutObjectInput{
				Bucket: aws.String(bucket),
				Key:    aws.String(key),
				Body:   file,
			})
			return out, err
		}

	} else {
		// Cas d'un dossier : la requête est traitée récursivement
		files, err := os.ReadDir(chemin)
		if err != nil {
			return nil, err
		}
		for _, file := range files {
			PutObject(client, chemin+"/"+file.Name(), bucket, key+"/"+file.Name())
		}
		return nil, err
	}
}

// TODO : gérer le cas des dossiers - faire des requetes parallèles ?
func traiterPut(client *client.S3EncryptionClientV3, reader *bufio.Reader) (*s3.PutObjectOutput, error) {
	// Idem : On récupère les infos de l'utilisateur pour appeller notre fonction
	fmt.Println("Vous avez demandé à mettre un fichier sur amazon S3 !")
	fmt.Print("- chemin du fichier/repertoire à mettre sur AWS S3 :  ")
	_, _ = reader.ReadString('\n')
	chemin, err := reader.ReadString('\n')
	if err != nil {
		fmt.Println("An error occured while reading input. Please try again", err)
		return nil, err
	}
	chemin = strings.TrimSuffix(chemin, "\n")
	// Ici, on récupère le chemin absolu pour que deux fichiers différents ne soient pas avec la même clé
	// absolutePath, err := filepath.Abs(chemin)
	fmt.Print("- nom du bucket S3 où mettre ce fichier :  ")
	// TODO : faire une option permettant d'afficher tous les buckets
	bucket, err := reader.ReadString('\n')
	if err != nil {
		fmt.Println("An error occured while reading input. Please try again", err)
		return nil, err
	}
	bucket = strings.TrimSuffix(bucket, "\n")
	// On regarde si le bucket est présent et sinon on en crée un
	// TODO : Vérifier que le bucket n'est pas déja présent sur S3 car sinon une erreur a lieu
	res, err := verifierBucket(client, bucket)
	if err != nil {
		fmt.Println("erreur lors de verifierBucket", err)
		return nil, err
	}
	if res == 1 {
		fmt.Println("Un nouveau bucket a été crée")
	} else if res == 0 {
		fmt.Println("L'action Put n'a pas été effectuée")
		return nil, err
	}

	fmt.Print("- sous-dossier dans lequel mettre le fichier/repertoire :  ")
	sous_rep, err := reader.ReadString('\n')
	if err != nil {
		fmt.Println("An error occured while reading input. Please try again", err)
		return nil, err
	}
	sous_rep = strings.TrimSuffix(sous_rep, "\n")
	// fmt.Printf("sous rep = %s\n",)
	// On regarde  si le dossier existe et on le crée si ce n'est pas le cas (si l'utilisateur est d'accord bien sur)
	if sous_rep != "" {
		res, err = verifierDossier(client, bucket, sous_rep)
		if res == 0 {
			fmt.Println("L'action Put n'a pas été effectuée")
			return nil, err
		}
	}

	fmt.Print("- nom à donner au fichier/dossier dans Amazon S3 :  ")
	key, err := reader.ReadString('\n')
	if err != nil {
		fmt.Println("An error occured while reading input. Please try again", err)
		return nil, err
	}
	key = strings.TrimSuffix(key, "\n")
	// Ici on vérifie quu'il n'y a pas déja un fichier avec le même emplacement, le choix est donné à l'utilisateur si il veut ou pas ecraser le fichier
	// TODO : possibilité d'avoir plusieurs versions d'un même fichier
	res = verifierKey(client, bucket, sous_rep+key)
	// fmt.Println("après verifierKey")
	if res == 1 {
		if sous_rep == "" {
			return PutObject(client, chemin, bucket, key)
		}
		return PutObject(client, chemin, bucket, sous_rep+"/"+key)
	} else {
		fmt.Println("L'action Put n'a pas été effectuée")
		return nil, err
	}
}
