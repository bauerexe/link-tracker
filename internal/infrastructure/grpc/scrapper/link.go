package scrappercontroller

import (
	"context"
	"errors"
	"strconv"
	"time"

	"go.uber.org/zap"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"

	usecase "gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/application/scrapper"
	"gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/domain"
	pbv1 "gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/pkg/api/proto"
)

func (a *api) CreateLink(ctx context.Context, req *pbv1.CreateLinkRequest) (*pbv1.LinkResponse, error) {
	if err := req.ValidateAll(); err != nil {
		return nil, status.Errorf(codes.InvalidArgument, "validation failed: %v", err)
	}

	chatID, err := chatIDFromReqOrMeta(ctx, req.GetChatId())
	if err != nil {
		return nil, err
	}

	if _, err = a.chatRepository.GetChatByID(ctx, chatID); err != nil {
		if errors.Is(err, usecase.ErrChatNotFound) {
			return nil, status.Errorf(codes.NotFound, "%s", err.Error())
		}
		a.log.Error("get chat failed", zap.Error(err))
		return nil, status.Errorf(codes.Internal, "internal error")
	}

	createdLink, err := a.linkRepository.CreateLink(ctx, chatID, req.GetLink(), req.GetTags(), req.GetFilters())
	if err != nil {
		if errors.Is(err, usecase.ErrLinkAlreadyTracked) {
			return nil, status.Errorf(codes.AlreadyExists, "%s", err.Error())
		}
		a.log.Error("create link failed", zap.Error(err))
		return nil, status.Errorf(codes.Internal, "internal error")
	}

	_ = a.linkRepository.SetURLState(ctx, createdLink.URL, domain.URLState{
		LastCheckedAt: time.Now(),
		LastUpdatedAt: time.Now(),
	})

	return &pbv1.LinkResponse{
		Id:      int32(createdLink.ID),
		Url:     createdLink.URL,
		Tags:    createdLink.Tags,
		Filters: createdLink.Filters,
	}, nil
}

func (a *api) GetLinks(ctx context.Context, req *pbv1.GetLinksRequest) (*pbv1.ListLinksResponse, error) {
	if err := req.ValidateAll(); err != nil {
		return nil, status.Errorf(codes.InvalidArgument, "validation failed: %v", err)
	}

	chatID, err := chatIDFromReqOrMeta(ctx, req.GetChatId())
	if err != nil {
		return nil, err
	}

	if _, err = a.chatRepository.GetChatByID(ctx, chatID); err != nil {
		if errors.Is(err, usecase.ErrChatNotFound) {
			return nil, status.Errorf(codes.NotFound, "%s", err.Error())
		}
		a.log.Error("get chat failed", zap.Error(err))
		return nil, status.Errorf(codes.Internal, "internal error")
	}

	gotLinks, err := a.linkRepository.GetLinksByChatID(ctx, chatID)
	if err != nil {
		if errors.Is(err, usecase.ErrChatNotFound) {
			return &pbv1.ListLinksResponse{
				Links: []*pbv1.LinkResponse{},
				Size:  0,
			}, nil
		}
		a.log.Error("get links failed", zap.Error(err))
		return nil, status.Errorf(codes.Internal, "internal error")
	}

	links := make([]*pbv1.LinkResponse, len(gotLinks))
	for i, link := range gotLinks {
		links[i] = &pbv1.LinkResponse{
			Id:      int32(link.ID),
			Url:     link.URL,
			Tags:    link.Tags,
			Filters: link.Filters,
		}
	}

	return &pbv1.ListLinksResponse{
		Links: links,
		Size:  int32(len(links)),
	}, nil
}

func (a *api) DeleteLink(ctx context.Context, req *pbv1.DeleteLinkRequest) (*pbv1.LinkResponse, error) {
	if err := req.ValidateAll(); err != nil {
		return nil, status.Errorf(codes.InvalidArgument, "validation failed: %v", err)
	}

	chatID, err := chatIDFromReqOrMeta(ctx, req.GetChatId())
	if err != nil {
		return nil, err
	}

	if _, err = a.chatRepository.GetChatByID(ctx, chatID); err != nil {
		if errors.Is(err, usecase.ErrChatNotFound) {
			return nil, status.Errorf(codes.NotFound, "%s", err.Error())
		}
		a.log.Error("get chat failed", zap.Error(err))
		return nil, status.Errorf(codes.Internal, "internal error")
	}

	deletedLink, err := a.linkRepository.DeleteLink(ctx, chatID, req.GetLink())
	if err != nil {
		if errors.Is(err, usecase.ErrChatNotFound) || errors.Is(err, usecase.ErrLinkNotFound) {
			return nil, status.Errorf(codes.NotFound, "%s", err.Error())
		}
		a.log.Error("delete link failed", zap.Error(err))
		return nil, status.Errorf(codes.Internal, "internal error")
	}

	return &pbv1.LinkResponse{
		Id:      int32(deletedLink.ID),
		Url:     deletedLink.URL,
		Tags:    deletedLink.Tags,
		Filters: deletedLink.Filters,
	}, nil
}

func chatIDFromReqOrMeta(ctx context.Context, reqChatID int64) (int64, error) {
	if reqChatID > 0 {
		return reqChatID, nil
	}

	md, ok := metadata.FromIncomingContext(ctx)
	if !ok {
		return 0, status.Errorf(codes.InvalidArgument, "missing Tg-Chat-Id")
	}

	vals := md.Get("tg-chat-id")
	if len(vals) == 0 {
		return 0, status.Errorf(codes.InvalidArgument, "missing Tg-Chat-Id")
	}

	id, err := strconv.ParseInt(vals[0], 10, 64)
	if err != nil || id <= 0 {
		return 0, status.Errorf(codes.InvalidArgument, "invalid Tg-Chat-Id")
	}

	return id, nil
}
