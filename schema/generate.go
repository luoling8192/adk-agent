package schema

//go:generate go run -mod=mod entgo.io/ent/cmd/ent generate ./ --target ../ent --feature=sql/execquery,sql/schemaconfig,sql/lock,sql/upsert
