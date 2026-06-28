package main

import (
	"fmt"
	"io"
	"mime"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/bootdotdev/learn-file-storage-s3-golang-starter/internal/auth"
	"github.com/google/uuid"
)

func (cfg *apiConfig) handlerUploadThumbnail(w http.ResponseWriter, r *http.Request) {
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

	fmt.Println("uploading thumbnail for video", videoID, "by user", userID)

	maxMem := 10 << 20
	err = r.ParseMultipartForm(int64(maxMem))

	file, fileHeader, err := r.FormFile("thumbnail")
	if err != nil {
		respondWithError(w, http.StatusBadRequest, "unable to get thumbnail from formdata", err)
		return
	}

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
	if mediaType != "image/jpeg" && mediaType != "image/png" {
		respondWithError(w, http.StatusBadRequest, "invalid media type. image/jpeg or image/png are allowed", err)
		return
	}

	video, err := cfg.db.GetVideo(videoID)
	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	if userID != video.UserID {
		respondWithError(w, http.StatusUnauthorized, "Couldn't validate JWT", err)
		return
	}
	contentSegs := strings.Split(contentType, "/")
	if len(contentSegs) < 1 {
		respondWithError(w, http.StatusBadRequest, "invalid content-type", err)
		return

	}
	fileSuffix := string(contentSegs[1])
	thumbFileName := fmt.Sprintf("%s.%s", videoID, fileSuffix)
	pathToFile := filepath.Join(cfg.assetsRoot, thumbFileName)
	ioFile, err := os.Create(pathToFile)
	if err != nil {
		respondWithError(w, http.StatusBadRequest, "failed to create file", err)
		return
	}

	_, err = io.Copy(ioFile, file)
	if err != nil {
		respondWithError(w, http.StatusBadRequest, "failed to write file", err)
		return
	}
	thumbUrl := fmt.Sprintf("http://localhost:%s/%s", cfg.port, pathToFile)
	video.ThumbnailURL = &thumbUrl
	err = cfg.db.UpdateVideo(video)
	if err != nil {
		respondWithError(w, http.StatusBadRequest, "unable to update video", err)
		return
	}

	respondWithJSON(w, http.StatusOK, video)
}
