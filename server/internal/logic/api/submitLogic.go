package api

import (
	"context"
	"database/sql"
	stdErrors "errors"
	"fmt"
	"github.com/zeromicro/go-zero/core/stores/sqlx"
	"github.com/zeromicro/x/errors"
	"net/http"
	"net/url"
	"server/internal/svc"
	"server/internal/types"
	"server/model"
	"time"

	"github.com/zeromicro/go-zero/core/logx"
)

type SubmitLogic struct {
	logx.Logger
	ctx        context.Context
	svcCtx     *svc.ServiceContext
	orderModel model.OrderModel
	photoModel model.PhotoModel
}

func NewSubmitLogic(ctx context.Context, svcCtx *svc.ServiceContext) *SubmitLogic {
	return &SubmitLogic{
		Logger:     logx.WithContext(ctx),
		ctx:        ctx,
		svcCtx:     svcCtx,
		orderModel: model.NewOrderModel(svcCtx.DB),
		photoModel: model.NewPhotoModel(svcCtx.DB),
	}
}

func (l *SubmitLogic) Submit(req *types.SubmitRequest) (resp *types.SubmitResponse, err error) {

	var (
		order       *model.Order
		result      sql.Result
		totalPhotos int64
		orderId     int64
	)

	order, err = l.orderModel.FindOneByOrderSn(l.ctx, req.OrderSn)
	if err != nil && !stdErrors.Is(err, sqlx.ErrNotFound) {
		return resp, err
	}

	// 如果订单存在，并且订单已经进入处理中状态，那么不允许用户重新上传
	if order != nil && order.Status == model.OrderStatusProcessing {
		return resp, errors.New(-1, "订单已经进入处理流程，无法重新上传图片，如有疑问请联系田田洗照片处理")
	}

	// 没有订单的话创建订单
	if order == nil {
		order = &model.Order{
			OrderSn:   req.OrderSn,
			Receiver:  req.Receiver,
			Remark:    req.Remark,
			Status:    model.OrderStatusPending,
			CreatedAt: time.Now(),
			UpdatedAt: time.Now(),
		}
		if result, err = l.orderModel.Insert(l.ctx, order); err != nil {
			return nil, err
		}
		if orderId, err = result.LastInsertId(); err != nil {
			return nil, err
		}
		order.Id = uint64(orderId)

	} else {
		order.Receiver = req.Receiver
		order.Remark = req.Remark
		order.UpdatedAt = time.Now()
		// 改为代处理状态，让系统重新同步一次
		order.Status = model.OrderStatusPending

		if err = l.orderModel.Update(l.ctx, order); err != nil {
			return nil, err
		}

		// 删除订单下关联的订单数据
		if err = l.photoModel.DeleteByOrderId(l.ctx, order.Id); err != nil {
			return nil, err
		}
	}

	// 统计每个规格的照片数量和调整状态
	specCount := make(map[string]int64)
	specResizedCount := make(map[string]int64)

	// 把照片数据关联给订单
	photos := make([]*model.Photo, 0)
	for _, photo := range req.Photos {
		// 添加每个URL对应的照片记录
		for _, metadata := range photo.Metadata {
			if metadata.URL == "" {
				continue
			}

			// CDN域名替换为源站域名
			parsedURL, err := url.Parse(metadata.URL)
			if err != nil {
				logx.Errorf("解析URL失败: %v", err)
				continue
			}

			originUrl := url.URL{
				Scheme:   l.svcCtx.Config.Minio.Schema,
				Host:     l.svcCtx.Config.Minio.Endpoint,
				Path:     parsedURL.Path,
				RawQuery: parsedURL.RawQuery,
			}

			p := &model.Photo{
				OrderId:   order.Id,
				Url:       metadata.URL,
				ThumbUrl:  metadata.URL, // 添加缩略图URL，暂时与原URL相同
				OriginUrl: originUrl.String(),
				Spec:      photo.Spec,
				Status:    model.PhotoStatusPending, // 设置为待处理状态
				Error:     "",                       // 初始化错误信息为空
				IsResized: metadata.IsResized,
				CreatedAt: time.Now(),
				UpdatedAt: time.Now(),
			}
			photos = append(photos, p)
			totalPhotos++
			specCount[photo.Spec]++
			if metadata.IsResized == 1 {
				specResizedCount[photo.Spec]++
			}
		}
	}

	for _, photo := range photos {
		if _, err = l.photoModel.Insert(l.ctx, photo); err != nil {
			logx.Errorf("插入照片记录失败: orderID=%d, url=%s, spec=%s, 错误: %v",
				photo.OrderId, photo.Url, photo.Spec, err)
			return nil, err
		}
	}

	logx.Infof("订单提交成功: orderSn=%s, receiverName=%s, 共%d张照片",
		req.OrderSn, req.Receiver, totalPhotos)

	// 发送 PushDeer 通知
	isNewOrder := order.Id == uint64(orderId) // 如果orderId不为0，说明是新创建的订单
	go l.sendPushDeerNotification(req, specCount, specResizedCount, totalPhotos, isNewOrder)

	resp = new(types.SubmitResponse)
	resp.Total = totalPhotos

	// 如果订单已经存在，那么删除订单下所有关联的photo
	return resp, nil
}

// 发送 PushDeer 通知
func (l *SubmitLogic) sendPushDeerNotification(req *types.SubmitRequest, specCount map[string]int64, specResizedCount map[string]int64, totalPhotos int64, isNewOrder bool) {
	// 从配置中获取 PushKeys
	pushKeys := l.svcCtx.Config.PushDeer.Keys
	if len(pushKeys) == 0 {
		logx.Alert("PushDeer 配置为空，跳过推送通知")
		return
	}

	// 根据是否为新订单设置不同的标题
	var title string
	if isNewOrder {
		title = "## 📸 用户首次照片上传\n\n"
	} else {
		title = "## 📝 订单修改通知\n\n"
	}

	// 构造 Markdown 格式消息内容
	message := title
	message += fmt.Sprintf("**📋 订单号:** %s\n\n", req.OrderSn)
	message += fmt.Sprintf("**👤 收货人:** %s\n\n", req.Receiver)
	if req.Remark != "" {
		message += fmt.Sprintf("**📝 订单备注:** %s\n\n", req.Remark)
	}
	message += "### 📊 照片详情\n\n"

	// 添加每个规格的照片数量和调整状态
	for spec, count := range specCount {
		resizedCount := specResizedCount[spec]
		var resizeStatus string

		if resizedCount == count {
			resizeStatus = "用户已全部调整"
		} else if resizedCount > 0 {
			resizeStatus = fmt.Sprintf("用户调整了%d张", resizedCount)
		} else {
			resizeStatus = "无调整"
		}

		message += fmt.Sprintf("- **%s:** %d张，尺寸：%s\n", spec, count, resizeStatus)
	}

	message += fmt.Sprintf("\n**📈 总计:** %d张照片\n\n", totalPhotos)
	message += fmt.Sprintf("### 🔗 照片地址：https://photo-kits.vencenty.cc/upload?order_sn=%s", req.OrderSn)

	// 向每个 PushKey 发送消息
	for _, pushKey := range pushKeys {
		go func(key string) {
			err := l.sendToPushDeer(key, message)
			if err != nil {
				logx.Errorf("发送 PushDeer 通知失败: pushKey=%s, 错误: %v", key, err)
			} else {
				logx.Infof("PushDeer 通知发送成功: pushKey=%s, orderSn=%s", key, req.OrderSn)
			}
		}(pushKey)
	}
}

// 发送消息到 PushDeer
func (l *SubmitLogic) sendToPushDeer(pushKey, message string) error {
	apiURL := "https://api2.pushdeer.com/message/push"

	// URL 编码消息内容
	params := url.Values{}
	params.Add("pushkey", pushKey)
	params.Add("text", "订单提醒")
	params.Add("desp", message)
	params.Add("type", "markdown")

	fullURL := fmt.Sprintf("%s?%s", apiURL, params.Encode())

	// 发送 GET 请求
	resp, err := http.Get(fullURL)
	if err != nil {
		return fmt.Errorf("发送 HTTP 请求失败: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("PushDeer API 返回错误状态码: %d", resp.StatusCode)
	}

	return nil
}
