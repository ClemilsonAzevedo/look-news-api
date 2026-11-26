package handler

import (
	"net/http"
	"reflect"

	"github.com/clemilsonazevedo/look-news/internal/feed"
	"github.com/getkin/kin-openapi/openapi3"
	"github.com/go-fuego/fuego"
	"github.com/go-fuego/fuego/option"
	"github.com/go-fuego/fuego/param"
)

type NewsRes struct {
	Articles []feed.Article `json:"articles"`
	Total    int            `json:"total"`
}

type Handler struct {
	srv       *fuego.Server
	cache     *feed.Cache
	refresher *feed.Refresher
}

func NewHandler(srv *fuego.Server, cache *feed.Cache, refresher *feed.Refresher) *Handler {
	return &Handler{srv: srv, cache: cache, refresher: refresher}
}

func (h *Handler) BindRoutes() {
	fuego.Get(h.srv, "/news", h.HandleNews,
		option.Summary("Listar notícias recentes"),
		option.OperationID("listNewsArticles"),
		option.TagInfo("Notícias", "Operações para consultar artigos recentes em feeds RSS, Atom e RDF."),
		option.OverrideDescription("Recebe uma ou mais URLs de feeds e combina os artigos publicados na semana corrente, de acordo com o fuso horário local do servidor. As fontes são processadas na ordem informada e os artigos mantêm a ordem de cada feed; não há ordenação global. Se uma fonte estiver indisponível, artigos armazenados em cache podem ser usados. Se todas as fontes falharem e não houver artigos em cache, a operação retorna 503. Como o servidor acessa as URLs informadas, envie apenas feeds públicos e confiáveis."),

		// query params
		option.QueryArray(
			"sources",
			"URL pública de um feed RSS, Atom ou RDF. Repita este parâmetro para cada fonte; os artigos serão combinados na ordem informada.",
			reflect.String,
			param.Required(),
			param.Example("Two feeds", []any{"https://example.com/feed.xml", "https://example.org/rss"}),
		),
		option.Header("If-None-Match", "ETag recebido anteriormente. Se o valor corresponder exatamente à representação atual, a resposta será 304 e não terá corpo.", param.Example("ETag atual", `"abc123"`)),

		option.AddResponse(http.StatusOK, "Consulta concluída. A lista pode estar vazia.", fuego.Response{
			Type:         NewsRes{},
			ContentTypes: []string{"application/json"},
		}),
		addNotModifiedResponse,
		option.ResponseHeader("ETag", "Identificador opaco da representação atual, usado em consultas condicionais.", param.StatusCodes(http.StatusOK, http.StatusNotModified), param.Example("ETag atual", `"abc123"`)),
		option.ResponseHeader("Cache-Control", "Diretivas de cache HTTP para respostas bem-sucedidas e validações condicionais.", param.StatusCodes(http.StatusOK, http.StatusNotModified), param.Example("Diretivas de cache", "max-age=60, stale-while-revalidate=300")),

		option.AddResponse(http.StatusBadRequest, "Parâmetro obrigatório ausente ou inválido.", fuego.Response{
			Type:         fuego.HTTPError{},
			ContentTypes: []string{"application/problem+json"},
		}),

		option.AddResponse(http.StatusInternalServerError, "Erro interno inesperado.", fuego.Response{
			Type:         fuego.HTTPError{},
			ContentTypes: []string{"application/problem+json"},
		}),

		option.AddResponse(http.StatusServiceUnavailable, "Todas as fontes falharam e não há artigos em cache disponíveis.", fuego.Response{
			Type:         fuego.HTTPError{},
			ContentTypes: []string{"application/problem+json"},
		}),
		addResponseExamples,
	)
}

func addNotModifiedResponse(route *fuego.BaseRoute) {
	route.Operation.AddResponse(
		http.StatusNotModified,
		openapi3.NewResponse().WithDescription("A representação corresponde ao valor informado em If-None-Match; a resposta não contém corpo."),
	)
}

func addResponseExamples(route *fuego.BaseRoute) {
	addResponseExample(route, "200", "application/json", "Resposta com artigos", map[string]any{
		"articles": []any{
			map[string]any{
				"title":     "Como os feeds estruturam notícias",
				"summary":   "Um resumo do artigo fornecido pela fonte.",
				"link":      "https://example.com/articles/news",
				"date":      "2026-11-26T12:00:00Z",
				"source":    "Example News",
				"author":    "Redação",
				"published": "Thu, 26 Nov 2026 12:00:00 GMT",
				"terms":     []string{"tecnologia"},
			},
		},
		"total": 1,
	})
	addResponseExample(route, "200", "application/json", "Sem artigos", map[string]any{
		"articles": nil,
		"total":    0,
	})
	addResponseExample(route, "400", "application/problem+json", "Parâmetro obrigatório ausente", map[string]any{
		"title":  "Query Param Not Found",
		"status": http.StatusBadRequest,
		"detail": "cannot parse request parameter: sources is a required query param",
	})
	addResponseExample(route, "500", "application/problem+json", "Erro interno", map[string]any{
		"title":  "Internal Server Error",
		"status": http.StatusInternalServerError,
		"detail": "An unexpected error occurred",
	})
	addResponseExample(route, "503", "application/problem+json", "Fontes indisponíveis", map[string]any{
		"title":  "sources unavailable",
		"status": http.StatusServiceUnavailable,
		"detail": "all sources failed, no cache available",
	})
}

func addResponseExample(route *fuego.BaseRoute, status, mediaType, name string, value any) {
	response := route.Operation.Responses.Value(status)
	if response == nil || response.Value == nil {
		return
	}

	content := response.Value.Content[mediaType]
	if content == nil {
		return
	}
	if content.Examples == nil {
		content.Examples = make(openapi3.Examples)
	}
	content.Examples[name] = &openapi3.ExampleRef{Value: openapi3.NewExample(value)}
}
