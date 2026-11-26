# Look News API

![Logo do Look News](assets/look-news-logo.png)

API HTTP em Go para buscar e servir artigos de feeds RSS, Atom e RDF.

[API em produção](https://look-news-api.onrender.com) · [Swagger interativo](https://look-news-api.onrender.com/swagger) · [Especificação OpenAPI](docs/openapi.json)


## Visão geral

A API agrega artigos de feeds RSS, Atom e RDF em uma resposta JSON uniforme. O contrato OpenAPI é publicado pelo servidor Fuego e mantido também em [`docs/openapi.json`](docs/openapi.json); use o Swagger para explorar os parâmetros, os modelos e as respostas por status HTTP.

### Comportamento

- Cada URL enviada em `sources` representa uma fonte. As fontes são processadas na ordem recebida e os artigos mantêm a ordem do feed; a API não faz ordenação global.
- São incluídos artigos com data válida entre o início da semana corrente (domingo, 00:00) e o momento da consulta, usando o fuso horário local do servidor.
- O cache de feeds é mantido em memória. Uma fonte já consultada é atualizada em segundo plano a cada hora; falhas podem usar os artigos anteriormente armazenados.
- Se todas as fontes falharem e nenhuma tiver conteúdo em cache, a API retorna `503 Service Unavailable`. Se não houver artigos elegíveis no período, a consulta ainda pode ser bem-sucedida sem resultados.
- O serviço busca as URLs fornecidas pelo cliente. Envie somente URLs públicas e confiáveis; não use endereços de rede privada ou serviços internos.

## Requisitos

- Go 1.26.6 ou superior.
- `make` para usar os comandos auxiliares do projeto (opcional).

## Executar localmente

Execute os comandos a partir da raiz do projeto. O `make` é opcional; ao rodá-lo, você verá os comandos disponíveis e suas descrições.

### Recomendado: usar o Makefile

```bash
make
make download
```

Opcionalmente, crie um arquivo `.env` para configurar a porta:

```env
PORT=8080
```

Inicie o servidor com:

```bash
make run
```

O Makefile também oferece `make build` para compilar a API e `make lint` para formatar o código e executar `go vet ./...`.

### Alternativa manual com Go

Se preferir executar os comandos Go diretamente, baixe as dependências e inicie o servidor:

```bash
go mod download
go run ./cmd/web
```

Por padrão, a API fica disponível em `http://localhost:8080`. A [documentação Swagger local](http://localhost:8080/swagger) permite explorar a rota.

## Configuração

| Variável | Padrão | Descrição |
| --- | --- | --- |
| `PORT` | `8080` | Porta HTTP do servidor. |

O arquivo `.env` é carregado quando existe na pasta de onde o servidor é iniciado. Em produção, configure as variáveis no ambiente do serviço e não versione credenciais.

## Endpoint

### `GET /news`

Busca os feeds informados e retorna os artigos publicados durante a semana corrente, conforme o horário local do servidor. Não é necessário enviar corpo na requisição.

| Parâmetro | Local | Obrigatório | Formato | Descrição |
| --- | --- | --- | --- | --- |
| `sources` | Query string | Sim | Texto, repetível | URL pública de um feed RSS, Atom ou RDF. Envie um parâmetro por fonte. |

Cada valor de `sources` deve ser uma URL acessível pelo servidor. A API consulta as fontes no servidor, não no navegador ou no cliente que chamou a API.

Exemplo com uma fonte:

```bash
curl --get 'http://localhost:8080/news' \
  --data-urlencode 'sources=https://example.com/feed.xml'
```

Exemplo com várias fontes:

```bash
curl --get 'http://localhost:8080/news' \
  --data-urlencode 'sources=https://example.com/feed.xml' \
  --data-urlencode 'sources=https://example.org/rss'
```

Use endereços de feeds públicos e confiáveis: as requisições às fontes são feitas pelo servidor.

### Resposta `200 OK`

```json
{
  "articles": [
    {
      "title": "Título do artigo",
      "summary": "Resumo do artigo",
      "link": "https://example.com/article",
      "date": "2026-11-26T12:00:00Z",
      "source": "Nome da fonte",
      "author": "Nome do autor",
      "published": "Thu, 26 Nov 2026 12:00:00 GMT",
      "terms": ["tecnologia"]
    }
  ],
  "total": 1
}
```

`date` é serializado em RFC 3339. Artigos sem data válida ou fora do intervalo da semana corrente não são incluídos. `published` preserva a data textual informada pela fonte; `source` identifica o feed, e `terms` contém as categorias disponíveis quando fornecidas.

### Códigos de resposta

| Código | Significado |
| --- | --- |
| `200` | Consulta concluída. A lista pode não conter artigos elegíveis. |
| `304` | A representação corresponde ao valor recebido em `If-None-Match`; não há corpo na resposta. |
| `400` | O parâmetro obrigatório `sources` não foi enviado. |
| `500` | Ocorreu um erro interno inesperado. |
| `503` | Todas as fontes falharam e não há artigos em cache disponíveis. |

As respostas de erro usam `application/problem+json` e incluem `title`, `status` e `detail`. Exemplo de `503`:

```json
{
  "title": "sources unavailable",
  "status": 503,
  "detail": "all sources failed, no cache available"
}
```

## Cache HTTP

As respostas `200` incluem `ETag` e `Cache-Control: max-age=60, stale-while-revalidate=300`. Quando a validação condicional não encontra alterações, a resposta `304` também inclui esses cabeçalhos, mas não contém corpo. Para validar uma representação já obtida, envie o `ETag` recebido anteriormente em `If-None-Match`:

```bash
curl -i --get 'http://localhost:8080/news' \
  --data-urlencode 'sources=https://example.com/feed.xml' \
  -H 'If-None-Match: "etag-recebido"'
```

Se o valor corresponder ao `ETag` atual, a resposta será `304 Not Modified`, sem corpo. O cache de feeds é mantido somente na memória e é perdido quando o processo reinicia. As fontes registradas são atualizadas periodicamente, em intervalos de uma hora; a primeira consulta a uma fonte sem cache busca o feed antes de responder.

## Desenvolvimento

Execute os comandos abaixo a partir da raiz do projeto:

```bash
go build -o look-news ./cmd/web
```

Execute os testes e a análise estática:

```bash
go test ./...
go vet ./...
```

## Ambientes

| Ambiente | Endereço |
| --- | --- |
| Local | `http://localhost:8080` |
| Produção | [API de produção](https://look-news-api.onrender.com) |
| Documentação de produção | [Swagger](https://look-news-api.onrender.com/swagger) |

O serviço de produção pode entrar em repouso após períodos sem tráfego; a primeira requisição depois desse intervalo pode demorar mais.

## Atualização da especificação

As descrições de operações, parâmetros, cabeçalhos, modelos e respostas são declaradas junto à rota Fuego em `internal/feed/http/handler/handler.go`. A configuração global (título, versão e descrição) fica em `cmd/api/api.go`. Ao iniciar o servidor, Fuego atualiza `docs/openapi.json`; revise esse arquivo quando alterar o contrato e mantenha os exemplos deste README alinhados com a implementação.