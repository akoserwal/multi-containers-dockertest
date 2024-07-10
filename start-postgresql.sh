docker network create sample-go-net || true

docker run \
  --name app-datastore \
  --net sample-go-net \
  --restart=always \
  -e POSTGRES_PASSWORD=$(cat secrets/db.password) \
  -e POSTGRES_USER=$(cat secrets/db.user) \
  -e POSTGRES_DB=$(cat secrets/db.name) \
  -p 5432:5432 \
  -d postgres:latest