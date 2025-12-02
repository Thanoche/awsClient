# AWS Client

## Fork 
Il s'agit d'un **fork** du client réalisé par **Maryam Munim**. Ce fork ajoute la possibilité de **générer dynamiquement une clé** pour chaque fichier. Nous générons une clé dérivée (CK) à partir de la clé d'origine. Pour le déchiffrement, il faut alors demander au HSM de recalculer cette clé à partir du CK. Fonctionnelle avec le [client Keystore TLS](https://github.com/anaelmessan/Keystore-TLS-client-python).

----

Implémentation d'un client AWS qui propose à l'utilisateur de mettre et récupérer des fichiers sur S3.

Il utilise le S3 encryption client pour chiffrer les fichiers côté client. Celui-ci fait des requêtes à un client HSM pour récupérer les clés voulues sur le HSM.

Pour tester le client AWS, il faut que le client HSM soit en train de tourner. On peut lancer le programme mockHSMclient.go pour simuler un client HSM qui traite les requêtes selon le format demandé.

# Prerequis

Go 1.23

# Utilisation

- Définir l'endpoint AWS. Pour cela le client AWS peut soit fonctionner avec un vrai compte AWS, soit avec [LocalStack](https://github.com/localstack/localstack).

    - **setup d'un compte AWS** Depuis la console AWS, créer une clé d'accès AWS. On obtient deux valeurs : l'id de la clé d'accès, et sa valeur secrète. Écrire les deux valeurs dans le fichier awsClient/.aws/credentials :
        ```
        [default]
        aws_access_key_id = AKIA***************
        aws_secret_access_key = ***************************************
        ```
        Par défault le client AWS va lire ce fichier et se connecter à ce compte.
    - **setup LocalStack** Après avoir installé LocalStack, il faut le lancer avec la commande ```localstack start```. Pour utiliser le client AWS avec LocalStack, il faudra ajouter l'arguement ```-localstack``` en lançant le programme. Par défaut l'endpoint est "http://localhost:4566"

- Lancer le client HSM. Si on n'a pas de client HSM, on peut tester avec le programme mockHSMclient.go. Depuis le répertoire mockHSMclient/ : ```go run mockHSMclient.go```. Le port par défaut est 6123.

- Lancer le client AWS. Depuis le répertoire awsClient/ ```go run cmd/awsClient/main.go```. On peut passer les arguements suivant :
    -HSMclient : port du client HSM (par défaut 6123)
    -localstack : mettre à true pour utiliser un endpoint LocalStack
    -h : afficher les arguments
