package scimprotocol

import (
	"fmt"
	"log"
	"net/http"

	"github.com/julienschmidt/httprouter"
	"github.com/memsql/errors"
	"github.com/muir/nchi"
	"github.com/muir/nject/v2"
	"github.com/muir/nvelope"
	"github.com/singlestore-labs/scim/scimerror"
	"github.com/singlestore-labs/scim/util"
)

type SCIMConnectionErr error

// https://datatracker.ietf.org/doc/html/rfc7644#section-3.2
func SCIMRouter(tracer util.Trace, server Server) func(*nchi.Mux) {
	return func(r *nchi.Mux) {
		annotateError := func(inner func() error, r *http.Request, requestBody nvelope.Body, endpoint nchi.Endpoint) error {
			// error kinds:
			// 		- ReturnCode(nil, 201)
			// 		- SCIMErr
			// 		- error
			// 		- nil
			requestErr := inner()
			if requestErr == nil {
				return nil
			}
			// skip if only status code
			if errors.Is(requestErr, scimerror.NoErrorReturning201) {
				return requestErr
			}

			tracer.Logf("[SCIM] Error from request url:%s, body:%s, caught: %s", r.URL, string(requestBody), requestErr)

			// wrap SCIM error to nvelope.ReturnCode to set code for response
			var scimErr *scimerror.SCIMError
			if errors.As(requestErr, &scimErr) {
				scimErr.Detail = errors.Redact(scimErr.Detail)
				return nvelope.ReturnCode(scimErr, scimErr.Status)
			} else {
				return scimerror.NewSCIMErr(500, errors.Redact(requestErr)) // return in json format to show in azure validation
			}
		}

		EncodeSCIMJSON := nvelope.MakeResponseEncoder("SCIM",
			nvelope.WithEncoder("application/scim+json", Marshal,
				nvelope.WithEncoderErrorTransform(func(err error) (interface{}, bool) {
					var sm SCIMMarshaler
					if errors.As(err, &sm) {
						return sm, true
					}
					return nil, false
				}),
			))

		DecodeSCIMJSON := nvelope.GenerateDecoder(
			nvelope.WithDecoder("application/json", Unmarshal),
			nvelope.WithDefaultContentType("application/json"),
			nvelope.WithPathVarsFunction(func(p httprouter.Params) nvelope.RouteVarLookup {
				return p.ByName
			}),
		)

		catchErrors := func(inner func() error, w *nvelope.DeferredWriter, endpoint nchi.Endpoint) {
			err := inner()
			if err == nil {
				return
			}

			tracer.Logf("[SCIM] Error from %s caught: %s", endpoint, err)
			if !w.Done() {
				http.Error(w, errors.Redact(err).Error(), 500)
			}
		}

		r.Use(
			nvelope.LoggerFromStd(log.Default()),
			nvelope.InjectWriter,
			nject.Provide("catch-errors", nject.Shun(catchErrors)),
			nvelope.CatchPanic,
			nvelope.ReadBody,
			DecodeSCIMJSON,
			EncodeSCIMJSON,
			annotateError,
			nvelope.Nil204,
			nject.Required(server.Authorization), // required for safe
		)

		r.Get("/ResourceTypes", server.GetResourceTypesHandler)
		r.Get("/Schemas", server.SchemasHandler)
		r.Get("/ServiceProviderConfig", server.GetServiceProviderConfigHandler)

		for _, rt := range server.GetResourceTypes() {
			resourceEndpoint := fmt.Sprintf("%s%s", rt.Endpoint, "/:resourceID")
			r.Get(rt.Endpoint, rt, server.GetResourceListHandler)
			r.Post(rt.Endpoint, rt, server.PostResourceHandler)
			r.Get(resourceEndpoint, rt, server.GetResourceHandler)
			r.Patch(resourceEndpoint, rt, server.PatchResourceHandler)
			r.Put(resourceEndpoint, rt, server.UpdateResourceHandler)
			r.Delete(resourceEndpoint, rt, server.DeleteResourceHandler)
		}
	}
}
