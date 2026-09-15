package httpapi

import (
	"fmt"
	"net/http"
	"strconv"

	"github.com/dronm/ds/v4"
	"github.com/dronm/meatshop/internal/integration1c"
	"github.com/dronm/meatshop/internal/services"
	"github.com/dronm/webapp"
)

func orderPrint1CHandler(db ds.Provider, client *integration1c.Client) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id, err := positiveOrderDocumentPathID(r)
		if err != nil {
			webapp.WriteBindingError(w, err)
			return
		}

		pdf, err := services.GetOrderPrint1C(r.Context(), db, client, id)
		if err != nil {
			webapp.WriteServiceError(w, err)
			return
		}

		fileName := fmt.Sprintf("order-%d.pdf", id)
		w.Header().Set("Content-Type", "application/pdf")
		w.Header().Set("Content-Disposition", fmt.Sprintf(`attachment; filename="%s"`, fileName))
		w.Header().Set("Content-Length", strconv.Itoa(len(pdf.Body)))
		w.Header().Set("Cache-Control", "no-store")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write(pdf.Body)
	}
}
