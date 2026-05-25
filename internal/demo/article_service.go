package demo

import (
	"context"
	"sort"
	"strings"
	"time"

	"github.com/itsolver/zentui/internal/types"
)

type ArticleService struct {
	store *Store
}

func NewArticleService(store *Store) *ArticleService {
	return &ArticleService{store: store}
}

func (s *ArticleService) List(ctx context.Context, opts *types.ListArticlesOptions) (*types.ArticlePage, error) {
	articles := demoArticles()
	sortArticles(articles, opts)
	page, meta := paginateArticles(articles, listArticleLimitAndCursor(opts))
	return &types.ArticlePage{
		Articles: page,
		Count:    len(articles),
		Meta:     meta,
	}, nil
}

func (s *ArticleService) Get(ctx context.Context, id int64) (*types.ArticleResult, error) {
	for _, article := range demoArticles() {
		if article.ID == id {
			return &types.ArticleResult{Article: article}, nil
		}
	}
	return nil, types.NewNotFoundError("demo article not found")
}

func (s *ArticleService) Search(ctx context.Context, query string, opts *types.SearchArticlesOptions) (*types.ArticleSearchPage, error) {
	query = strings.ToLower(strings.TrimSpace(query))
	var results []types.Article
	for _, article := range demoArticles() {
		haystack := strings.ToLower(article.Title + "\n" + article.Body + "\n" + strings.Join(article.LabelNames, "\n"))
		if query == "" || strings.Contains(haystack, query) {
			results = append(results, article)
		}
	}
	page, meta := paginateArticles(results, searchArticleLimitAndCursor(opts))
	return &types.ArticleSearchPage{
		Results: page,
		Count:   len(results),
		Meta:    meta,
	}, nil
}

type articlePageOptions struct {
	limit  int
	cursor string
}

func listArticleLimitAndCursor(opts *types.ListArticlesOptions) articlePageOptions {
	if opts == nil {
		return articlePageOptions{}
	}
	return articlePageOptions{limit: opts.Limit, cursor: opts.Cursor}
}

func searchArticleLimitAndCursor(opts *types.SearchArticlesOptions) articlePageOptions {
	if opts == nil {
		return articlePageOptions{}
	}
	return articlePageOptions{limit: opts.Limit, cursor: opts.Cursor}
}

func sortArticles(articles []types.Article, opts *types.ListArticlesOptions) {
	if opts == nil || opts.SortBy == "" {
		return
	}
	desc := strings.EqualFold(opts.SortOrder, "desc")
	less := func(i, j int) bool {
		switch opts.SortBy {
		case "created_at":
			return articles[i].CreatedAt.Before(articles[j].CreatedAt)
		case "updated_at":
			return articles[i].UpdatedAt.Before(articles[j].UpdatedAt)
		case "title":
			return strings.ToLower(articles[i].Title) < strings.ToLower(articles[j].Title)
		default:
			return articles[i].ID < articles[j].ID
		}
	}
	sort.SliceStable(articles, func(i, j int) bool {
		if desc {
			return less(j, i)
		}
		return less(i, j)
	})
}

func paginateArticles(articles []types.Article, opts articlePageOptions) ([]types.Article, types.PageMeta) {
	limit := 25
	if opts.limit > 0 {
		limit = opts.limit
	}
	offset := 0
	if opts.cursor != "" {
		offset = decodeCursor(opts.cursor)
	}
	if offset < 0 {
		offset = 0
	}
	if offset > len(articles) {
		offset = len(articles)
	}
	end := offset + limit
	hasMore := end < len(articles)
	if end > len(articles) {
		end = len(articles)
	}
	var afterCursor string
	if hasMore {
		afterCursor = encodeCursor(end)
	}
	return articles[offset:end], types.PageMeta{HasMore: hasMore, AfterCursor: afterCursor}
}

func demoArticles() []types.Article {
	now := time.Date(2026, 1, 15, 10, 0, 0, 0, time.UTC)
	return []types.Article{
		{
			ID:         9001,
			Title:      "Resetting multi-factor authentication",
			Body:       "Steps for verifying identity and resetting MFA for a customer account.",
			AuthorID:   1001,
			SectionID:  7001,
			CreatedAt:  now.AddDate(0, -3, 0),
			UpdatedAt:  now,
			LabelNames: []string{"mfa", "login", "security"},
			Locale:     "en-us",
			HTMLURL:    "https://" + DemoSubdomain + ".zendesk.com/hc/en-us/articles/9001",
		},
		{
			ID:         9002,
			Title:      "Troubleshooting invoice delivery",
			Body:       "Common checks for missing billing emails, spam filtering, and account contacts.",
			AuthorID:   1002,
			SectionID:  7002,
			CreatedAt:  now.AddDate(0, -2, 0),
			UpdatedAt:  now.AddDate(0, 0, -2),
			Promoted:   true,
			LabelNames: []string{"billing", "invoice", "email"},
			Locale:     "en-us",
			HTMLURL:    "https://" + DemoSubdomain + ".zendesk.com/hc/en-us/articles/9002",
		},
	}
}
