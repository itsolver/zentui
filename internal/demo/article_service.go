package demo

import (
	"context"
	"fmt"
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
	return &types.ArticlePage{
		Articles: articles,
		Count:    len(articles),
		Meta:     types.PageMeta{HasMore: false},
	}, nil
}

func (s *ArticleService) Get(ctx context.Context, id int64) (*types.ArticleResult, error) {
	for _, article := range demoArticles() {
		if article.ID == id {
			return &types.ArticleResult{Article: article}, nil
		}
	}
	return nil, fmt.Errorf("demo article not found: %d", id)
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
	return &types.ArticleSearchPage{
		Results: results,
		Count:   len(results),
		Meta:    types.PageMeta{HasMore: false},
	}, nil
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
