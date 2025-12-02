package awsClient

import (
	"bufio"
	"context"
	"fmt"
	"log"
	"os"
	"strconv"
	"strings"
	"time"

	MyMaterials "awsClient/pkg/awsEncryptionMaterials"

	"github.com/aws/amazon-s3-encryption-client-go/v3/client"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/feature/s3/manager"
	"github.com/aws/aws-sdk-go-v2/service/s3"
)

// Ce fichier s'occupe de la réimplémentation de GetObject

// Alors en théorie, cette fonction peut récupérer des dossiers de manière récursive grace à la structure arborescente Node (voir tree.go)
func GetObject(client *client.S3EncryptionClientV3, root *Node, chemin, bucket, key string) (*s3.GetObjectOutput, error) {
	// On mesure le temps que met l'action GetObject
	start := time.Now()
	defer func() {
		duration := time.Since(start)
		fmt.Printf("L'action Get a pris %v\n", duration)
	}()
	if root.IsFile {
		// fmt.Println("cas 1 (fichier)")
		// fmt.Printf("chemin =%s, bucket = %s, key =%s\n", chemin, bucket, key)

		// test taille
		headObject, err := client.HeadObject(context.TODO(), &s3.HeadObjectInput{
			Bucket: aws.String(bucket),
			Key:    aws.String(key),
		})
		if err != nil {
			fmt.Println("Erreur lors de la récupération des métadonnées :", err)
			return nil, err
		}
		// fmt.Printf("taille fichier : %d octets\n", int(*headObject.ContentLength))

		ctx := context.TODO()
		ctx = context.WithValue(ctx, MyMaterials.FavContextKey("x"), []byte(bucket+"/"+key))
		fmt.Println(MyMaterials.FavContextKey("x"))
		// cas d'un gros fichier (on utilise alors le downloader, similairement avec le PUT et l'uploader)
		// TODO : ça marche pas, erreur "clé inconnue"
		if int(*headObject.ContentLength) > 500*1000*1000 {
			downloader := manager.NewDownloader(client, func(u *manager.Downloader) { // ici on prend un *client.S3EncryptionClientV3
				u.PartSize = 10 * 1024 * 1024 // 1MB per part
			})

			// création d'un fichier
			file, err := os.Create(chemin)
			if err != nil {
				return nil, fmt.Errorf("erreur lors de la création du fichier local : %w", err)
			}
			defer file.Close()

			// download
			fmt.Printf("clé :\"%s\"\n", key)
			numBytesDownloaded, err := downloader.Download(ctx, file, &s3.GetObjectInput{
				Bucket: aws.String(bucket),
				Key:    aws.String(key),
			})
			if err != nil {
				fmt.Println("Erreur lors du download multipart :", err)
				return nil, err
			}

			fmt.Printf("Téléchargé %d bytes depuis S3 et écrit dans %s\n", numBytesDownloaded, chemin)
			return nil, nil

		} else { // cas d'un petit fichier (on fait un simple GET)
			out, err := client.GetObject(ctx, &s3.GetObjectInput{
				Bucket: aws.String(bucket),
				Key:    aws.String(key),
			})
			if err != nil {
				log.Fatalf("erreur avec appel de GetObject\n")
			}
			meta := out.Metadata
			// On récupère dans les métadonnées du fichier sa taille
			taille, err := strconv.Atoi(meta["x-amz-unencrypted-content-length"])
			if err != nil {
				log.Fatalf("Problème dans la récupération de la taille du fichier: %v", err)
			}
			// on crée un fichier de cette taille
			p := make([]byte, taille)
			Body := out.Body
			_, err = Body.Read(p)
			if err != nil {
				log.Fatalf("%v", err)
			}
			err = os.WriteFile(chemin, p[:], 0o666)
			if err != nil {
				log.Fatalf("problème dans l'écriture du fichier%v", err)
			}
			return out, err
		}
	} else {
		// Cas dossier : on crée un nouveau dossier et on appelle récursivement la fonction GetObject
		fmt.Println("cas 2")
		newPath := chemin + "/" + root.Name
		err := os.Mkdir(newPath, 0o666)
		if err != nil {
			return nil, err
		}
		for newKey, newRoot := range root.Children {
			newPath = newPath + newKey
			GetObject(client, newRoot, newPath, bucket, key+"/"+newKey)
		}
		return nil, err
	}
}

func traiterGet(client *client.S3EncryptionClientV3, reader *bufio.Reader) (*s3.GetObjectOutput, error) {
	// Fonction pour l'interface avec l'utilisateur : on lui demande toutes les infos nécéssaire à la fonction getObject
	// TODO : gérer les erreurs
	fmt.Println("vous avez demandé à récupérer un fichier sur amazon S3 !")

	fmt.Println("Veuillez indiquer le bucket où se trouve ce fichier")
	// TODO : faire une option permettant d'afficher tous les buckets et renvoyer une erreur si le bucket n'existe pas	bucket, err := reader.ReadString('\n')
	_, _ = reader.ReadString('\n')
	// Pour une raison inconnue, la première fois, la lecture n'est pas effectuée
	bucket, err := reader.ReadString('\n')
	if err != nil {
		fmt.Println("An error occured while reading input. Please try again", err)
		return nil, err
	}
	bucket = strings.TrimSuffix(bucket, "\n")
	// Cette fonction se trouve dans tree.go et permet d'organiser en arborescence la demande du client

	estPresent := bucketPresent(client, bucket)
	if !estPresent {
		fmt.Println("le bucket que vous avez demandé n'existe pas, l'action Get ne sera pas effectuée")
		return nil, err
	}

	fmt.Println("Quel est le chemin dans Amazon S3 du fichier que vous voulez récupérer ?")
	key, err := reader.ReadString('\n')
	if err != nil {
		fmt.Println("An error occured while reading input. Please try again", err)
		return nil, err
	}
	key = strings.TrimSuffix(key, "\n")
	estPresent, root := inAWSS3(client, bucket, key)
	if !estPresent {
		fmt.Println("le fichier n'existe pas")
	}
	fmt.Println("Veuillez indiquer l'emplacement (local) désiré pour le fichier/dossier")
	chemin, err := reader.ReadString('\n')
	if err != nil {
		fmt.Println("An error occured while reading input. Please try again", err)
		return nil, err
	}
	chemin = strings.TrimSuffix(chemin, "\n")

	return GetObject(client, root, chemin, bucket, key)
}
