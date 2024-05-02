package line

import (
	"context"
	"strconv"

	"github.com/pkg/errors"
	"github.com/superj80820/system-design/domain"
	loggerKit "github.com/superj80820/system-design/kit/logger"
)

type actressLineUseCase struct {
	actressLineRepo     domain.ActressLineRepository
	facePlusPlusUseCase domain.FacePlusPlusUseCase
	actressRepo         domain.ActressRepo
	lineMessageRepo     domain.LineMessageAPIRepo
	lineTemplateRepo    domain.LineTemplateRepo
	liffURI             string
	logger              loggerKit.Logger
	maxReplyCount       int
}

func CreateActressUseCase(
	actressRepo domain.ActressRepo,
	actressLineRepo domain.ActressLineRepository,
	lineMessageRepo domain.LineMessageAPIRepo,
	lineTemplateRepo domain.LineTemplateRepo,
	facePlusPlusUseCase domain.FacePlusPlusUseCase,
	liffURI string,
	logger loggerKit.Logger,
	maxReplyCount int,
) domain.ActressLineUseCase {
	return &actressLineUseCase{
		actressRepo:         actressRepo,
		actressLineRepo:     actressLineRepo,
		lineMessageRepo:     lineMessageRepo,
		lineTemplateRepo:    lineTemplateRepo,
		facePlusPlusUseCase: facePlusPlusUseCase,
		liffURI:             liffURI,
		logger:              logger,
		maxReplyCount:       maxReplyCount,
	}
}

func (a *actressLineUseCase) EnableGroupRecognition(ctx context.Context, groupID string, replyToken string) error {
	if err := a.actressLineRepo.EnableGroupRecognition(ctx, groupID); err != nil {
		return errors.Wrap(err, "enable group recognition failed")
	}
	if err := a.lineMessageRepo.Reply(replyToken, a.lineTemplateRepo.ApplyText("我準備好辨識惹～")); err != nil {
		return errors.Wrap(err, "reply text failed")
	}
	return nil
}

func (a *actressLineUseCase) GetUseInformation(replyToken string) error {
	information, err := a.actressLineRepo.GetUseInformation()
	if err != nil {
		return errors.Wrap(err, "get user information failed")
	}
	if err := a.lineMessageRepo.Reply(replyToken, a.lineTemplateRepo.ApplyText(information)); err != nil {
		return errors.Wrap(err, "reply text failed")
	}
	return nil
}

func (a *actressLineUseCase) GetWish(replyToken string) error {
	wish, err := a.actressRepo.GetWish()
	if err != nil {
		return errors.Wrap(err, "get user wish failed")
	}

	wishContent, err := a.lineTemplateRepo.ApplyTemplate("wish_content", wish.Preview, wish.Name, a.liffURI+"/?actressID="+wish.ID)
	if err != nil {
		return errors.Wrap(err, "apply template failed")
	}

	replyMessage, err := a.lineTemplateRepo.ApplyTemplate("wish", []string{wishContent})
	if err != nil {
		return errors.Wrap(err, "apply template failed")
	}

	if err := a.lineMessageRepo.Reply(replyToken, replyMessage); err != nil {
		return errors.Wrap(err, "reply text failed")
	}
	return nil
}

func (a *actressLineUseCase) RecognitionByGroup(ctx context.Context, groupID string, imageID string, replyToken string) error {
	isEnableGroupRecognition, err := a.actressLineRepo.IsEnableGroupRecognition(ctx, groupID)
	if err != nil {
		return errors.Wrap(err, "get enable group recognition status failed")
	}
	if !isEnableGroupRecognition {
		return nil
	}

	image, err := a.lineMessageRepo.GetImage(imageID)
	if err != nil {
		return errors.Wrap(err, "get line image failed")
	}

	faceInformation, err := a.facePlusPlusUseCase.SearchAllFaceSets(image)
	if errors.Is(err, domain.ErrRateLimit) {
		if err := a.lineMessageRepo.Reply(replyToken, a.lineTemplateRepo.ApplyText("伺服器繁忙中QwQ")); err != nil {
			return errors.Wrap(err, "reply text failed")
		}
		return nil
	} else if err != nil {
		return errors.Wrap(err, "recognition failed")
	}

	recognitionContents := make([]string, 0, a.maxReplyCount)
	for i := 0; i < a.maxReplyCount && i < len(faceInformation.SearchResults); i++ {
		serachResults := faceInformation.SearchResults[i]

		recognitionInformation, err := a.actressRepo.GetActressByFaceToken(serachResults.FaceToken)
		// TODO: york delete no data case
		if err != nil {
			return errors.Wrap(err, "recognition failed")
		}

		detail := "希望有幫你找到"
		if recognitionInformation.Detail != "" {
			detail = recognitionInformation.Detail
		}
		recognitionContent, err := a.lineTemplateRepo.ApplyTemplate("recognition_content", recognitionInformation.Preview, recognitionInformation.Name, detail, strconv.Itoa(int(serachResults.Confidence))+"%", a.liffURI+"/?actressID="+recognitionInformation.ID)
		if err != nil {
			return errors.Wrap(err, "apply template failed")
		}

		recognitionContents = append(recognitionContents, recognitionContent)
	}

	replyMessage, err := a.lineTemplateRepo.ApplyTemplate("recognition", recognitionContents)
	if err != nil {
		return errors.Wrap(err, "apply template failed")
	}

	if err := a.actressLineRepo.DisableGroupRecognition(ctx, groupID); err != nil { // TODO: maybe need tx
		return errors.Wrap(err, "disable group recognition failed")
	}

	if err := a.lineMessageRepo.Reply(replyToken, replyMessage, a.lineTemplateRepo.ApplyText("辨識已關閉～")); err != nil {
		return errors.Wrap(err, "reply text failed")
	}
	return nil
}

func (a *actressLineUseCase) RecognitionByUser(ctx context.Context, imageID string, replyToken string) error {
	image, err := a.lineMessageRepo.GetImage(imageID)
	if err != nil {
		return errors.Wrap(err, "get line image failed")
	}

	faceInformation, err := a.facePlusPlusUseCase.SearchAllFaceSets(image)
	if errors.Is(err, domain.ErrRateLimit) {
		a.logger.Debug("recognition get rate limit error", loggerKit.Error(err))
		if err := a.lineMessageRepo.Reply(replyToken, a.lineTemplateRepo.ApplyText("伺服器繁忙中QwQ")); err != nil {
			return errors.Wrap(err, "reply text failed")
		}
		return nil
	} else if err != nil {
		return errors.Wrap(err, "recognition failed")
	}

	recognitionContents := make([]string, 0, a.maxReplyCount)
	for i := 0; i < a.maxReplyCount && i < len(faceInformation.SearchResults); i++ {
		serachResults := faceInformation.SearchResults[i]

		recognitionInformation, err := a.actressRepo.GetActressByFaceToken(serachResults.FaceToken)
		// TODO: york delete no data case
		if err != nil {
			return errors.Wrap(err, "recognition failed")
		}

		detail := "希望有幫你找到"
		if recognitionInformation.Detail != "" {
			detail = recognitionInformation.Detail
		}
		recognitionContent, err := a.lineTemplateRepo.ApplyTemplate("recognition_content", recognitionInformation.Preview, recognitionInformation.Name, detail, strconv.Itoa(int(serachResults.Confidence))+"%", a.liffURI+"/?actressID="+recognitionInformation.ID)
		if err != nil {
			return errors.Wrap(err, "apply template failed")
		}

		recognitionContents = append(recognitionContents, recognitionContent)
	}

	replyMessage, err := a.lineTemplateRepo.ApplyTemplate("recognition", recognitionContents)
	if err != nil {
		return errors.Wrap(err, "apply template failed")
	}

	if err := a.lineMessageRepo.Reply(replyToken, replyMessage); err != nil {
		return errors.Wrap(err, "reply text failed")
	}
	return nil
}
