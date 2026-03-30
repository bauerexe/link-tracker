package scrappercontroller

import (
	"context"
	"errors"

	"go.uber.org/zap"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	usecase "gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/application/scrapper"
	pbv1 "gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/pkg/api/proto"
)

func (a *api) CreateChat(ctx context.Context, req *pbv1.CreateChatRequest) (*pbv1.ChatResponse, error) {
	if err := req.ValidateAll(); err != nil {
		return nil, status.Errorf(codes.InvalidArgument, "validation failed: %v", err)
	}

	createdChat, err := a.chatRepository.CreateChat(ctx, req.Id)
	if err != nil {
		if errors.Is(err, usecase.ErrChatAlreadyExist) {
			return nil, status.Errorf(codes.AlreadyExists, "%s", err.Error())
		}
		a.log.Error("create chat failed", zap.Error(err))
		return nil, status.Errorf(codes.Internal, "internal error")
	}

	return &pbv1.ChatResponse{Id: createdChat.ID}, nil
}

func (a *api) DeleteChat(ctx context.Context, req *pbv1.DeleteChatRequest) (*pbv1.ChatResponse, error) {
	if err := req.ValidateAll(); err != nil {
		return nil, status.Errorf(codes.InvalidArgument, "validation failed: %v", err)
	}

	deletedChat, err := a.chatRepository.DeleteChatByID(ctx, req.Id)
	if err != nil {
		if errors.Is(err, usecase.ErrChatNotFound) {
			return nil, status.Errorf(codes.NotFound, "%s", err.Error())
		}
		a.log.Error("delete chat failed", zap.Error(err))
		return nil, status.Errorf(codes.Internal, "internal error")
	}

	return &pbv1.ChatResponse{Id: deletedChat.ID}, nil
}
