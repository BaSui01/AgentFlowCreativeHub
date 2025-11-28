package content

import (
	"context"
	"errors"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

// ListComments 获取作品评论列表
func (s *Service) ListComments(ctx context.Context, tenantID, workID string, page, pageSize int) (*CommentListResponse, error) {
	var comments []WorkComment
	var total int64
	
	offset := (page - 1) * pageSize
	
	query := s.db.WithContext(ctx).
		Where("tenant_id = ? AND work_id = ? AND deleted_at IS NULL", tenantID, workID).
		Order("created_at DESC")
	
	// 统计总数
	if err := query.Model(&WorkComment{}).Count(&total).Error; err != nil {
		return nil, err
	}
	
	// 查询评论列表
	if err := query.Offset(offset).Limit(pageSize).Find(&comments).Error; err != nil {
		return nil, err
	}
	
	// 填充用户信息（可选，这里简化处理）
	// TODO: 通过JOIN或者批量查询填充username和avatar
	
	return &CommentListResponse{
		Comments: comments,
		Total:    total,
		Page:     page,
		PageSize: pageSize,
		HasMore:  int64(page*pageSize) < total,
	}, nil
}

// CreateComment 发表评论
func (s *Service) CreateComment(ctx context.Context, tenantID, userID string, req *CreateCommentRequest) (*WorkComment, error) {
	// 验证作品是否存在
	var work PublishedWork
	if err := s.db.WithContext(ctx).Where("id = ? AND tenant_id = ?", req.WorkID, tenantID).First(&work).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, ErrWorkNotFound
		}
		return nil, err
	}
	
	// 检查是否允许评论
	if !work.AllowComment {
		return nil, errors.New("该作品不允许评论")
	}
	
	// 如果有回复评论，验证回复的评论是否存在
	if req.ReplyToID != nil {
		var replyComment WorkComment
		if err := s.db.WithContext(ctx).Where("id = ? AND tenant_id = ? AND work_id = ?", 
			*req.ReplyToID, tenantID, req.WorkID).First(&replyComment).Error; err != nil {
			return nil, errors.New("回复的评论不存在")
		}
	}
	
	comment := &WorkComment{
		ID:        uuid.New().String(),
		TenantID:  tenantID,
		WorkID:    req.WorkID,
		UserID:    userID,
		Content:   req.Content,
		ReplyToID: req.ReplyToID,
		LikeCount: 0,
	}
	
	if err := s.db.WithContext(ctx).Create(comment).Error; err != nil {
		return nil, err
	}
	
	// TODO: 触发WebSocket通知作品作者
	
	return comment, nil
}

// DeleteComment 删除评论
func (s *Service) DeleteComment(ctx context.Context, tenantID, userID, commentID string) error {
	// 查找评论
	var comment WorkComment
	if err := s.db.WithContext(ctx).
		Where("id = ? AND tenant_id = ? AND deleted_at IS NULL", commentID, tenantID).
		First(&comment).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return errors.New("评论不存在")
		}
		return err
	}

	// 检查权限：只有评论作者可以删除
	// TODO: 添加管理员权限检查
	if comment.UserID != userID {
		return errors.New("无权删除此评论")
	}

	// 软删除评论
	if err := s.db.WithContext(ctx).Delete(&comment).Error; err != nil {
		return err
	}

	return nil
}

// ToggleLike 点赞/取消点赞作品
func (s *Service) ToggleLike(ctx context.Context, tenantID, userID, workID string) (*LikeResponse, error) {
	// 验证作品是否存在
	var work PublishedWork
	if err := s.db.WithContext(ctx).Where("id = ? AND tenant_id = ?", workID, tenantID).First(&work).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, ErrWorkNotFound
		}
		return nil, err
	}
	
	// 检查是否已点赞
	var existingLike WorkLike
	err := s.db.WithContext(ctx).Where("work_id = ? AND user_id = ? AND tenant_id = ?", 
		workID, userID, tenantID).First(&existingLike).Error
	
	isLiked := false
	
	if err == gorm.ErrRecordNotFound {
		// 未点赞，创建点赞记录
		like := &WorkLike{
			ID:       uuid.New().String(),
			TenantID: tenantID,
			WorkID:   workID,
			UserID:   userID,
		}
		
		if err := s.db.WithContext(ctx).Create(like).Error; err != nil {
			return nil, err
		}
		isLiked = true
	} else if err == nil {
		// 已点赞，删除点赞记录
		if err := s.db.WithContext(ctx).Delete(&existingLike).Error; err != nil {
			return nil, err
		}
		isLiked = false
	} else {
		return nil, err
	}
	
	// 重新查询点赞数（由数据库触发器自动更新）
	if err := s.db.WithContext(ctx).Where("id = ?", workID).First(&work).Error; err != nil {
		return nil, err
	}
	
	return &LikeResponse{
		IsLiked:   isLiked,
		LikeCount: work.LikeCount,
	}, nil
}
