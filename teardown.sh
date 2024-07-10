set -e

docker stop app-datastore

docker rm app-datastore

docker network rm sample-go-net