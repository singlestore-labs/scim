package example

import (
	"context"
	"io"
	"net/http"
	"reflect"
	"strconv"
	"strings"

	"github.com/memsql/errors"
	"github.com/muir/nvelope"
	scimprotocol "github.com/singlestore-labs/scim"
	"github.com/singlestore-labs/scim/scimerror"
	"github.com/singlestore-labs/scim/util"
)

func GetResourceHelper[T scimprotocol.Resource](
	r *http.Request,
	resourceID string,
	getResourceData func(ctx context.Context, resourceID string) (T, error),
) (nvelope.Response, error) {
	data, err := getResourceData(r.Context(), resourceID)
	if err != nil {
		return nil, err
	}
	attributes := toStrArray(r.URL.Query().Get("attributes"))
	excludedAttributes := toStrArray(r.URL.Query().Get("excludedAttributes"))

	if len(excludedAttributes) > 0 {
		attributes = excludedAttributes
	}
	result, err := scimprotocol.ResourceMarshal(data, attributes, len(excludedAttributes) > 0)
	return result, err
}

func GetListResourceHelper[T scimprotocol.Resource](
	r *http.Request,
	trace util.Trace,
	itemsPerPage int,
	getAllResourceData func(ctx context.Context) ([]T, error),
) (nvelope.Response, error) {
	startIndex := 1 // default startIndex is 1
	inputStartIndex := r.URL.Query().Get("startIndex")
	if inputStartIndex != "" {
		inputStart, err := strconv.Atoi(inputStartIndex)
		if err != nil {
			return nil, scimerror.NewBadRequestSCIMErr(scimerror.InvalidSyntax, errors.Wrapf(err, "invalid input startIndex, %s", inputStartIndex))
		}
		if inputStart > 1 {
			startIndex = inputStart
		}
	}

	count := itemsPerPage // default count is itemsPerPage
	inputCount := r.URL.Query().Get("count")
	if inputCount != "" {
		inputCount, err := strconv.Atoi(inputCount)
		if err != nil {
			return nil, scimerror.NewBadRequestSCIMErr(scimerror.InvalidSyntax, errors.Wrapf(err, "invalid input count, %s", inputCount))
		}
		count = inputCount
		if count < 0 {
			count = 0
		}
	}

	var filter *scimprotocol.OrExpression
	filterStr := r.URL.Query().Get("filter")
	if len(filterStr) > 0 {
		var err error
		filter, err = scimprotocol.ParseFilter(filterStr)
		if err != nil {
			return nil, err
		}
	}

	all, err := getAllResourceData(r.Context())
	if err != nil {
		return false, errors.Wrapf(err, "failed to get all resource (%T) data from db in SCIM", all)
	}
	filtered, err := scimprotocol.GetFilteredResources(filter, all)
	if err != nil {
		return nil, err
	}
	totalResultNum := len(filtered)

	// return current page data
	currentPageResources, err := getCurrentPageResources(filtered, startIndex, count)
	if err != nil {
		return nil, err
	}

	// select attributes
	attributes := toStrArray(r.URL.Query().Get("attributes"))
	excludedAttributes := toStrArray(r.URL.Query().Get("excludedAttributes"))
	if len(excludedAttributes) > 0 {
		attributes = excludedAttributes
	}

	return scimprotocol.MarshalWithSelectedAttr(scimprotocol.ListResponse[T]{
		Resources:    currentPageResources,
		ItemsPerPage: count,
		StartIndex:   startIndex,
		TotalResults: totalResultNum,
	}, attributes, len(excludedAttributes) > 0)
}

// getCurrentPageResources get the resources for current page according to startIndex and count
// Input: startIndex MUST >= 1, count MUST >= 0
func getCurrentPageResources[T scimprotocol.Resource](inputResource []T, startIndex, count int) ([]T, error) {
	resultStart := 0
	resultEnd := len(inputResource)

	targetStart := startIndex - 1
	targetEnd := startIndex + count - 1
	if count == 0 || targetStart > resultEnd {
		return []T{}, nil
	}

	if targetStart > 0 {
		resultStart = targetStart
	}

	if targetEnd < resultEnd && targetEnd > 0 {
		resultEnd = targetEnd
	}

	return inputResource[resultStart:resultEnd], nil
}

func PatchResourceHelper[T scimprotocol.Resource, IDType string](r *http.Request, resourceID IDType,
	getResourceFromDB func(ctx context.Context, resourceID IDType) (T, error),
	updateResourceToDB func(ctx context.Context, resourceID IDType, _ T) (T, error),
) (nvelope.Response, error) {
	// unmarshal patch operations
	b, err := io.ReadAll(r.Body)
	if err != nil {
		return nil, scimerror.NewSCIMErr(http.StatusBadRequest, errors.Wrapf(err, "could not get request body"))
	}
	patchOps, err := scimprotocol.UnmarshalPatchRequest(b)
	if err != nil {
		return nil, err
	}
	// get user
	resource, err := getResourceFromDB(r.Context(), resourceID)
	if err != nil {
		return nil, scimerror.NewSCIMErr(http.StatusNotFound, errors.Wrapf(err, "invalid input resource id, %s", resourceID))
	}
	// patch user with ops
	for _, op := range patchOps {
		path, err := scimprotocol.ParsePath(op.Path)
		if err != nil {
			return nil, err
		}
		coreSchema, _, err := scimprotocol.GetSchemaURIFromResource(reflect.TypeOf(resource), nil)
		if err != nil {
			return nil, err
		}
		pathNode := path.GetNodes(coreSchema)
		err = scimprotocol.Patch(reflect.ValueOf(&resource).Elem(), pathNode, op.Op, op.Value)
		if err != nil {
			return nil, err
		}
	}
	// db upsert user with response
	return updateResourceToDB(r.Context(), resourceID, resource)
}

func CreateResourceHelper[T scimprotocol.Resource](r *http.Request,
	createResourceToDB func(_ context.Context, _ T) (T, error),
) (nvelope.Response, error) {
	b, err := io.ReadAll(r.Body)
	if err != nil {
		return nil, scimerror.NewSCIMErr(http.StatusBadRequest, errors.Wrapf(err, "could not get request body"))
	}
	var resource T
	err = scimprotocol.Unmarshal(b, &resource)
	if err != nil {
		return nil, scimerror.NewSCIMErr(http.StatusBadRequest, errors.Wrapf(err, "could not unmarshal scim resource from request (body:%s)", string(b)))
	}
	resource, err = createResourceToDB(r.Context(), resource)
	if err == nil {
		err = scimerror.NoErrorReturning201
	}
	return resource, err
}

func UpdateResourceHelper[T scimprotocol.Resource](r *http.Request, resourceID string,
	updateResourceToDB func(_ context.Context, resourceID string, _ T) (T, error),
) (nvelope.Response, error) {
	b, err := io.ReadAll(r.Body)
	if err != nil {
		return nil, scimerror.NewSCIMErr(http.StatusBadRequest, errors.Wrapf(err, "could not get request body"))
	}
	var resource T
	err = scimprotocol.Unmarshal(b, &resource)
	if err != nil {
		return nil, scimerror.NewSCIMErr(http.StatusBadRequest, errors.Wrapf(err, "could not unmarshal scim resource from request (body:%s)", string(b)))
	}
	return updateResourceToDB(r.Context(), resourceID, resource)
}

func GetSchemasHelper(resourceTypes []scimprotocol.ResourceType, itemsPerPage int) (nvelope.Response, error) {
	schemas := []scimprotocol.Schema{}
	for _, rt := range resourceTypes {
		resourceSchemas, err := scimprotocol.GetResourceSchema(rt.ResourceObjectType)
		if err != nil {
			return nil, err
		}
		schemas = append(schemas, resourceSchemas...)
	}
	return scimprotocol.ListResponse[scimprotocol.Schema]{
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
