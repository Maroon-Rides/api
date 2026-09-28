data "external_schema" "bun" {
  program = ["go", "run", "-mod=mod", "./internal/db/loader"]
}

env "local" {
  src = data.external_schema.bun.url
  dev = "docker://postgres/18/dev?search_path=public"

  migration {
    dir    = "file://internal/db/migrations"
    format = goose
  }

  format {
    migrate {
      diff = "{{ sql . \"  \" }}"
    }
  }
}
