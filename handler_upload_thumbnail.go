package main

import (
	"fmt"
	"io"
	"net/http"

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
	contentType := header.Header.Get("Content-Type")

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

	// Save the thumbnail to the global map by creating new thumbnail struct with image data and media type
	tn := thumbnail{
		data:      thumbnailData,
		mediaType: contentType,
	}
	// add thumbnail to global map `videoThumbnails` using video's ID as the key
	videoThumbnails[videoID] = tn

	// Update database so that the existing video record has a new thumbnail URL by using
	// `cfg.db.UpdateVideo` function. The thumbnail URL should have this format:
	// `http://localhost:<port>/api/thumbnails/{videoID}` this works since the
	// `/api/thumbnails/{videoID}` enpoint serves thumbnails from the global map
	thumbnailURL := fmt.Sprintf("http://localhost:%d/api/thumbnails/%s", cfg.port, videoID)

	// Respond with updated JSON of the video's metadata using `respondWithJSON` function
	// and pass it the updated `database.Video` struct to marshal.
	video.ThumbnailURL = &thumbnailURL
    if video.ThumbnailURL == nil || *video.ThumbnailURL != thumbnailURL {
        respondWithError(w, http.StatusInternalServerError, "Thumbnail URL not set", err)
        return
    }
	err = cfg.db.UpdateVideo(video)
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "Couldn't update video", err)
		return
	}

	respondWithJSON(w, http.StatusOK, video)
}
