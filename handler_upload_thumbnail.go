package main

import (
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"

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

	const maxMemory = 10 << 20
	err = r.ParseMultipartForm(maxMemory)
	if err != nil {
		respondWithError(w, http.StatusBadRequest, "Unable to parse form", err)
		return
	}

	file, header, err := r.FormFile("thumbnail")
	if err != nil {
		respondWithError(w, http.StatusBadRequest, "Unable to parse thumbnail", err)
		return
	}
	defer file.Close()

	// get media type from the file's `Content-Type` header
	// should only be `image/png` or `image/jpeg` resolving to `png` or `jpeg`
	contentType := header.Header.Get("Content-Type")
	switch contentType {
	case "image/png":
		contentType = "png"
	case "image/jpeg":
		contentType = "jpeg"
	default:
		respondWithError(w, http.StatusBadRequest, "Invalid thumbnail type", nil)
		return
	}

	// read all image data into byte slice using `io.ReadAll`
	thumbnailData, err := io.ReadAll(file)
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "Couldn't read thumbnail", err)
		return
	}

	// get video's metadata from SQLite database. `apiConfig.db` has a `GetVideo` method
	video, err := cfg.db.GetVideo(videoID)
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "Couldn't get video", err)
		return
	}
	// if authenticated user is not the video owner, return `http.StatusUnauthorized` response
	if video.UserID != userID {
		respondWithError(w, http.StatusUnauthorized, "Unauthorized", nil)
		return
	}

	// save bytes to a file at the path `/assets/<videoID>.<file_extension>` using filepath.Join
	thumbnailPath := filepath.Join("assets", fmt.Sprintf("%s.%s", videoID, contentType))
	thumbnailFile, err := os.Create(thumbnailPath)
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "Couldn't create thumbnail file", err)
		return
	}
	defer thumbnailFile.Close()

	_, err = thumbnailFile.Write(thumbnailData)
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "Couldn't write thumbnail file", err)
		return
	}

	// update thumbnail_url to `http://localhost:<port>/assets/<videoID>.<file_extension>`
	tn_url := fmt.Sprintf("http://localhost:%s/assets/%s.%s", cfg.port, videoID, contentType)
	video.ThumbnailURL = &tn_url

	err = cfg.db.UpdateVideo(video)
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "Couldn't update video", err)
		return
	}

	respondWithJSON(w, http.StatusOK, video)
}
