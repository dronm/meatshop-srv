#one, smoke
go test ./internal/apitest -tags=integration -count=1 -v -run TestAuthSmoke

#all
go test ./internal/apitest -tags=integration -count=1 -v

#one
go test ./internal/apitest -tags=integration -run '^TestProductFilterByKind$' -count=1 -v

#one image
API_TEST_REMOVE_WATERMARKS=1 \
go test -tags=integration ./internal/apitest -run TestProductImageRemoveWatermarksByImageID -count=1

export SESSION_ID=""

curl -v \
	-X PATCH \
	-b "_s=$SESSION_ID" \
	"http://127.0.0.1:59000/api/product-images/7dd2c0f480464a0d0c3279715f240191/remove-watermarks"
