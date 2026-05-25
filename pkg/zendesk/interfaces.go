package zendesk

import (
	"context"

	"github.com/itsolver/zentui/internal/types"
)

//go:generate mockgen -destination=../../internal/mocks/mock_zendesk.go -package=mocks github.com/itsolver/zentui/pkg/zendesk TicketService,SearchService,UserService,ArticleService

type TicketService interface {
	List(ctx context.Context, opts *types.ListTicketsOptions) (*types.TicketPage, error)
	ListView(ctx context.Context, viewID int64, opts *types.ListTicketsOptions) (*types.TicketPage, error)
	Get(ctx context.Context, id int64, opts *types.GetTicketOptions) (*types.TicketResult, error)
	Create(ctx context.Context, req *types.CreateTicketRequest) (*types.Ticket, error)
	Update(ctx context.Context, id int64, req *types.UpdateTicketRequest) (*types.Ticket, error)
	Delete(ctx context.Context, id int64) error
	ListComments(ctx context.Context, ticketID int64, opts *types.ListCommentsOptions) (*types.CommentPage, error)
	ListAudits(ctx context.Context, ticketID int64, opts *types.ListAuditsOptions) (*types.AuditPage, error)
	ListTicketFields(ctx context.Context, opts *types.ListTicketFieldsOptions) (*types.TicketFieldPage, error)
	MergeTickets(ctx context.Context, targetID int64, req *types.MergeTicketsRequest) (*types.MergeTicketsResult, error)
}

type SearchService interface {
	Search(ctx context.Context, query string, opts *types.SearchOptions) (*types.SearchPage, error)
}

type UserService interface {
	GetMe(ctx context.Context) (*types.User, error)
	Get(ctx context.Context, id int64) (*types.User, error)
	AutocompleteUsers(ctx context.Context, name string) ([]types.User, error)
	ListIdentities(ctx context.Context, userID int64, opts *types.ListUserIdentitiesOptions) (*types.UserIdentityPage, error)
	CreateIdentity(ctx context.Context, userID int64, req *types.CreateUserIdentityRequest) (*types.UserIdentity, error)
	MergeEndUser(ctx context.Context, sourceUserID int64, targetUserID int64) (*types.JobStatus, error)
}

type ArticleService interface {
	List(ctx context.Context, opts *types.ListArticlesOptions) (*types.ArticlePage, error)
	Get(ctx context.Context, id int64) (*types.ArticleResult, error)
	Search(ctx context.Context, query string, opts *types.SearchArticlesOptions) (*types.ArticleSearchPage, error)
}
