package scimprotocol

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"reflect"
	"strconv"
	"strings"
	"time"

	"github.com/memsql/errors"
	"github.com/muir/nvelope"

	"singlestore.com/helios/scim/scimmetrics"
	"singlestore.com/helios/scim/scimprotocol/scimerror"
	"singlestore.com/helios/trace"
	"singlestore.com/helios/uuid"
)

func GetResourceHelper[T Resource, IDType uuid.SCIMResource](
	r *http.Request,
	scimID uuid.SCIM,
	resourceID IDType,
	getResouceData func(ctx context.Context, scimID uuid.SCIM, resourceID IDType) (T, error),
) (nvelope.Response, error) {
	data, err := getResouceData(r.Context(), scimID, resourceID)
	if err != nil {
		return nil, err
	}
	attributes := toStrArray(r.URL.Query().Get("attributes"))
	excludedAttributes := toStrArray(r.URL.Query().Get("excludedAttributes"))

	if len(excludedAttributes) > 0 {
		attributes = excludedAttributes
	}
	result, err := ResourceMarshal(data, attributes, len(excludedAttributes) > 0)
	return result, err
}

func GetListResourceHelper[T Resource](
	r *http.Request,
	trace trace.Iface,
	scimID uuid.SCIM,
	itemsPerPage int,
	getAllResourceData func(ctx context.Context, scimID uuid.SCIM) ([]T, error),
) (nvelope.Response, error) {
	filterStr := r.URL.Query().Get("filter")
	attributes := toStrArray(r.URL.Query().Get("attributes"))
	excludedAttributes := toStrArray(r.URL.Query().Get("excludedAttributes"))
	inputStartIndex := r.URL.Query().Get("startIndex")
	inputCount := r.URL.Query().Get("count")

	startIndex := 1
	if inputStartIndex != "" {
		inputStart, err := strconv.Atoi(inputStartIndex)
		if err != nil {
			return nil, err
		}
		if inputStart > 1 {
			startIndex = inputStart
		}
	}
	count := itemsPerPage
	if inputCount != "" {
		inputCount, err := strconv.Atoi(inputCount)
		if err != nil {
			return nil, err
		}
		if inputCount > 0 {
			count = inputCount
		}
	}

	var filter *OrExpression
	if len(filterStr) > 0 {
		var err error
		filter, err = ParseFilter(filterStr)
		if err != nil {
			return nil, err
		}
	}

	startProcess := time.Now()
	checkFilter := func(resource T) (bool, error) {
		if filter != nil {
			return filter.Eval(reflect.ValueOf(resource), false)
		}
		return true, nil
	}
	// TODO:MCDB-63978 pass filter to get user function while doing row.Next()
	all, err := getAllResourceData(r.Context(), scimID)
	if err != nil {
		return false, errors.Wrapf(err, "failed to get all resource (%T) data from db in SCIM (%s)", all, scimID)
	}
	filtered := []T{}
	for _, u := range all {
		pass, err := checkFilter(u)
		if err != nil {
			return false, err
		}
		if pass {
			filtered = append(filtered, u)
		}
	}

	if scimmetrics.GateSCIMMetrics.Enabled() && filter != nil {
		scimmetrics.RequestsFilterDuration.
			WithLabelValues(scimID.String(), string(scimmetrics.GetIDPFromHeader(r.Header)), filter.RedactedString()).
			Observe(float64(time.Since(startProcess)) / float64(time.Millisecond))
	}

	// only return current page data
	var currentPageResources []T
	totalResultNum := len(filtered)
	if startIndex <= len(filtered) {
		resultStart := startIndex - 1
		resultEnd := startIndex + count - 1
		if resultEnd > len(filtered) {
			resultEnd = len(filtered)
		}
		currentPageResources = filtered[resultStart:resultEnd]
	}

	// clean return data
	var result []json.RawMessage
	if len(attributes) > 0 || len(excludedAttributes) > 0 {
		for _, u := range currentPageResources {
			if len(excludedAttributes) > 0 {
				attributes = excludedAttributes
			}
			j, err := ResourceMarshal(u, attributes, len(excludedAttributes) > 0)
			if err != nil {
				return nil, err
			}
			result = append(result, j)
		}
	} else {
		for _, u := range currentPageResources {
			j, err := Marshal(u)
			if err != nil {
				return nil, err
			}
			result = append(result, j)
		}
	}
	return ListResponse[json.RawMessage]{
		Resources:    result,
		ItemsPerPage: count,
		StartIndex:   startIndex,
		TotalResults: totalResultNum,
	}, nil
}

func PatchResourceHelper[T Resource, IDType uuid.SCIMResource](r *http.Request, scimID uuid.SCIM, resourceID IDType,
	getResourceFromDB func(ctx context.Context, scimID uuid.SCIM, resourceID IDType) (T, error),
	updateResourceToDB func(ctx context.Context, scimID uuid.SCIM, resourceID IDType, _ T) (T, error),
) (nvelope.Response, error) {
	// unmarshal patch operations
	b, err := io.ReadAll(r.Body)
	if err != nil {
		return nil, scimerror.NewSCIMErr(http.StatusBadRequest, errors.Wrapf(err, "could not get request body"))
	}
	patchOps, err := UnmarshalPatchRequest(b)
	if err != nil {
		return nil, err
	}
	// get user
	resource, err := getResourceFromDB(r.Context(), scimID, resourceID)
	if err != nil {
		return nil, scimerror.NewSCIMErr(http.StatusNotFound, errors.Wrapf(err, "invalid input resource id, %s", resourceID))
	}
	// patch user with ops
	for _, op := range patchOps {
		path, err := ParsePath(op.Path)
		if err != nil {
			return nil, err
		}
		coreSchema, _, err := GetSchemaURIFromResource(reflect.TypeOf(resource), nil)
		if err != nil {
			return nil, err
		}
		pathNode := path.GetNodes(coreSchema)
		err = Patch(reflect.ValueOf(&resource).Elem(), pathNode, op.Op, op.Value)
		if err != nil {
			return nil, err
		}
	}
	// db upsert user with response
	return updateResourceToDB(r.Context(), scimID, resourceID, resource)
}

func CreateResourceHelper[T Resource](r *http.Request, scimID uuid.SCIM,
	createResourceToDB func(_ context.Context, scimID uuid.SCIM, _ T) (T, error),
) (nvelope.Response, error) {
	b, err := io.ReadAll(r.Body)
	if err != nil {
		return nil, scimerror.NewSCIMErr(http.StatusBadRequest, errors.Wrapf(err, "could not get request body"))
	}
	var resource T
	err = Unmarshal(b, &resource)
	if err != nil {
		return nil, scimerror.NewSCIMErr(http.StatusBadRequest, errors.Wrapf(err, "could not unmarshal scim resource from request (body:%s)", string(b)))
	}
	resource, err = createResourceToDB(r.Context(), scimID, resource)
	if err == nil {
		err = scimerror.NoErrorReturning201
	}
	return resource, err
}

func UpdateResourceHelper[T Resource, IDType uuid.SCIMResource](r *http.Request, scimID uuid.SCIM, resourceID IDType,
	updateResourceToDB func(_ context.Context, scimID uuid.SCIM, resourceID IDType, _ T) (T, error),
) (nvelope.Response, error) {
	b, err := io.ReadAll(r.Body)
	if err != nil {
		return nil, scimerror.NewSCIMErr(http.StatusBadRequest, errors.Wrapf(err, "could not get request body"))
	}
	var resource T
	err = Unmarshal(b, &resource)
	if err != nil {
		return nil, scimerror.NewSCIMErr(http.StatusBadRequest, errors.Wrapf(err, "could not unmarshal scim resource from request (body:%s)", string(b)))
	}
	return updateResourceToDB(r.Context(), scimID, resourceID, resource)
}

func GetSchemasHelper(resourceTypes []ResourceType, itemsPerPage int) (nvelope.Response, error) {
	schemas := []Schema{}
	for _, rt := range resourceTypes {
		resourceSchemas, err := GetResourceSchema(rt.ResourceObjectType)
		if err != nil {
			return nil, err
		}
		schemas = append(schemas, resourceSchemas...)
	}
	return ListResponse[Schema]{
		StartIndex:   1,
		ItemsPerPage: itemsPerPage,
		Resources:    schemas,
		TotalResults: len(schemas),
	}, nil
}

func toStrArray(queryStr string) []string {
	var attributes []string
	if queryStr != "" {
		attributes = strings.Split(queryStr, ",")
	}
	return attributes
}
