package main

import (
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"io"
	"mime"
	"net/http"
	"os"

	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/bootdotdev/learn-file-storage-s3-golang-starter/internal/auth"
	"github.com/google/uuid"
)

func (cfg *apiConfig) handlerUploadVideo(w http.ResponseWriter, r *http.Request) {
	videoIDString := r.PathValue("videoID")
	videoID, err := uuid.Parse(videoIDString)
	if err != nil {
		respondWithError(w, http.StatusBadRequest, "Invalid ID", err)
		return
	}

	token, err := auth.GetBearerToken(r.Header)
	if err != nil {
		respondWithError(w, http.StatusUnauthorized, "Couldn't find JWT", err)
		return
	}

	userID, err := auth.ValidateJWT(token, cfg.jwtSecret)
	if err != nil {
		respondWithError(w, http.StatusUnauthorized, "Couldn't validate JWT", err)
		return
	}

	video, err := cfg.db.GetVideo(videoID)
	if err != nil {
		respondWithError(w, http.StatusNotFound, "video not found", err)
		return
	}
	if video.UserID != userID {
		respondWithError(w, http.StatusUnauthorized, "unauthorized", err)
		return
	}

	maxMem := 10 << 30
	err = r.ParseMultipartForm(int64(maxMem))

	file, fileHeader, err := r.FormFile("video")
	if err != nil {
		respondWithError(w, http.StatusBadRequest, "unable to get video from form", err)
		return
	}
	defer file.Close()

	contentType := fileHeader.Header.Get("content-type")
	if contentType == "" {
		respondWithError(w, http.StatusBadRequest, "emtpy content type", err)
		return
	}
	mediaType, _, err := mime.ParseMediaType(contentType)
	if err != nil {
		respondWithError(w, http.StatusBadRequest, "failed parsing mediatype", err)
		return

	}
	if mediaType != "video/mp4" {
		respondWithError(w, http.StatusBadRequest, "invalid media type. expected video/mp4", err)
		return
	}

	temp, err := os.CreateTemp("", "upload-mp4")
	defer os.Remove(temp.Name())
	defer temp.Close()
	if err != nil {
		respondWithError(w, http.StatusBadRequest, "failed to open temp file", err)
		return
	}
	_, err = io.Copy(temp, file)
	if err != nil {
		respondWithError(w, http.StatusBadRequest, "failed to write to temp file", err)
		return
	}
	temp.Seek(0, io.SeekStart)

	rdm := make([]byte, 32)
	_, _ = rand.Read(rdm)

	rdmId := base64.RawURLEncoding.EncodeToString(rdm)
	fileKey := fmt.Sprintf("%s.mp4", rdmId)
	opts := s3.PutObjectInput{
		Bucket:      &cfg.s3Bucket,
		Key:         &fileKey,
		Body:        temp,
		ContentType: &mediaType,
	}
	_, err = cfg.s3Client.PutObject(r.Context(), &opts)
	if err != nil {
		respondWithError(w, http.StatusBadRequest, "failed to put object", err)
		return
	}

	videoUrl := fmt.Sprintf("https://%s.s3.%s.amazonaws.com/%s", cfg.s3Bucket, cfg.s3Region, fileKey)
	video.VideoURL = &videoUrl
	err = cfg.db.UpdateVideo(video)
	if err != nil {
		respondWithError(w, http.StatusBadRequest, "failed to updated video entry", err)
		return
	}

	respondWithJSON(w, http.StatusOK, video)
}
