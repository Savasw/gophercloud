package routers

import (
	"fmt"
	"strconv"

	"github.com/racker/perigee"
	"github.com/rackspace/gophercloud"
	"github.com/rackspace/gophercloud/openstack/utils"
	"github.com/rackspace/gophercloud/pagination"
)

// ListOpts allows the filtering and sorting of paginated collections through
// the API. Filtering is achieved by passing in struct field values that map to
// the floating IP attributes you want to see returned. SortKey allows you to
// sort by a particular network attribute. SortDir sets the direction, and is
// either `asc' or `desc'. Marker and Limit are used for pagination.
type ListOpts struct {
	ID           string
	Name         string
	AdminStateUp *bool
	Status       string
	TenantID     string
	Limit        int
	Marker       string
	SortKey      string
	SortDir      string
}

// List returns a Pager which allows you to iterate over a collection of
// routers. It accepts a ListOpts struct, which allows you to filter and sort
// the returned collection for greater efficiency.
//
// Default policy settings return only those routers that are owned by the
// tenant who submits the request, unless an admin user submits the request.
func List(c *gophercloud.ServiceClient, opts ListOpts) pagination.Pager {
	q := make(map[string]string)
	if opts.ID != "" {
		q["id"] = opts.ID
	}
	if opts.Name != "" {
		q["name"] = opts.Name
	}
	if opts.AdminStateUp != nil {
		q["admin_state_up"] = strconv.FormatBool(*opts.AdminStateUp)
	}
	if opts.Status != "" {
		q["status"] = opts.Status
	}
	if opts.TenantID != "" {
		q["tenant_id"] = opts.TenantID
	}
	if opts.Marker != "" {
		q["marker"] = opts.Marker
	}
	if opts.Limit != 0 {
		q["limit"] = strconv.Itoa(opts.Limit)
	}
	if opts.SortKey != "" {
		q["sort_key"] = opts.SortKey
	}
	if opts.SortDir != "" {
		q["sort_dir"] = opts.SortDir
	}

	u := rootURL(c) + utils.BuildQuery(q)
	return pagination.NewPager(c, u, func(r pagination.LastHTTPResponse) pagination.Page {
		return RouterPage{pagination.LinkedPageBase{LastHTTPResponse: r}}
	})
}

// CreateOpts contains all the values needed to create a new router. There are
// no required values.
type CreateOpts struct {
	Name         string
	AdminStateUp *bool
	TenantID     string
	GatewayInfo  *GatewayInfo
}

// Create accepts a CreateOpts struct and uses the values to create a new
// logical router. When it is created, the router does not have an internal
// interface - it is not associated to any subnet.
//
// You can optionally specify an external gateway for a router using the
// GatewayInfo struct. The external gateway for the router must be plugged into
// an external network (it is external if its `router:external' field is set to
// true).
func Create(c *gophercloud.ServiceClient, opts CreateOpts) CreateResult {
	type router struct {
		Name         *string      `json:"name,omitempty"`
		AdminStateUp *bool        `json:"admin_state_up,omitempty"`
		TenantID     *string      `json:"tenant_id,omitempty"`
		GatewayInfo  *GatewayInfo `json:"external_gateway_info,omitempty"`
	}

	type request struct {
		Router router `json:"router"`
	}

	reqBody := request{Router: router{
		Name:         gophercloud.MaybeString(opts.Name),
		AdminStateUp: opts.AdminStateUp,
		TenantID:     gophercloud.MaybeString(opts.TenantID),
	}}

	if opts.GatewayInfo != nil {
		reqBody.Router.GatewayInfo = opts.GatewayInfo
	}

	var res CreateResult
	_, err := perigee.Request("POST", rootURL(c), perigee.Options{
		MoreHeaders: c.Provider.AuthenticatedHeaders(),
		ReqBody:     &reqBody,
		Results:     &res.Resp,
		OkCodes:     []int{201},
	})
	res.Err = err
	return res
}

// Get retrieves a particular router based on its unique ID.
func Get(c *gophercloud.ServiceClient, id string) GetResult {
	var res GetResult
	_, err := perigee.Request("GET", resourceURL(c, id), perigee.Options{
		MoreHeaders: c.Provider.AuthenticatedHeaders(),
		Results:     &res.Resp,
		OkCodes:     []int{200},
	})
	res.Err = err
	return res
}

// UpdateOpts contains the values used when updating a router.
type UpdateOpts struct {
	Name         string
	AdminStateUp *bool
	GatewayInfo  *GatewayInfo
}

// Update allows routers to be updated. You can update the name, administrative
// state, and the external gateway. For more information about how to set the
// external gateway for a router, see Create. This operation does not enable
// the update of router interfaces. To do this, use the AddInterface and
// RemoveInterface functions.
func Update(c *gophercloud.ServiceClient, id string, opts UpdateOpts) UpdateResult {
	type router struct {
		Name         *string      `json:"name,omitempty"`
		AdminStateUp *bool        `json:"admin_state_up,omitempty"`
		GatewayInfo  *GatewayInfo `json:"external_gateway_info,omitempty"`
	}

	type request struct {
		Router router `json:"router"`
	}

	reqBody := request{Router: router{
		Name:         gophercloud.MaybeString(opts.Name),
		AdminStateUp: opts.AdminStateUp,
	}}

	if opts.GatewayInfo != nil {
		reqBody.Router.GatewayInfo = opts.GatewayInfo
	}

	// Send request to API
	var res UpdateResult
	_, err := perigee.Request("PUT", resourceURL(c, id), perigee.Options{
		MoreHeaders: c.Provider.AuthenticatedHeaders(),
		ReqBody:     &reqBody,
		Results:     &res.Resp,
		OkCodes:     []int{200},
	})
	res.Err = err
	return res
}

// Delete will permanently delete a particular router based on its unique ID.
func Delete(c *gophercloud.ServiceClient, id string) DeleteResult {
	var res DeleteResult
	_, err := perigee.Request("DELETE", resourceURL(c, id), perigee.Options{
		MoreHeaders: c.Provider.AuthenticatedHeaders(),
		OkCodes:     []int{204},
	})
	res.Err = err
	return res
}

var errInvalidInterfaceOpts = fmt.Errorf("When adding a router interface you must provide either a subnet ID or a port ID")

// InterfaceOpts allow you to work with operations that either add or remote
// an internal interface from a router.
type InterfaceOpts struct {
	SubnetID string
	PortID   string
}

// AddInterface attaches a subnet to an internal router interface. You must
// specify either a SubnetID or PortID in the request body. If you specify both,
// the operation will fail and an error will be returned.
//
// If you specify a SubnetID, the gateway IP address for that particular subnet
// is used to create the router interface. Alternatively, if you specify a
// PortID, the IP address associated with the port is used to create the router
// interface.
//
// If you reference a port that is associated with multiple IP addresses, or
// if the port is associated with zero IP addresses, the operation will fail and
// a 400 Bad Request error will be returned.
//
// If you reference a port already in use, the operation will fail and a 409
// Conflict error will be returned.
//
// The PortID that is returned after using Extract() on the result of this
// operation can either be the same PortID passed in or, on the other hand, the
// identifier of a new port created by this operation. After the operation
// completes, the device ID of the port is set to the router ID, and the
// device owner attribute is set to `network:router_interface'.
func AddInterface(c *gophercloud.ServiceClient, id string, opts InterfaceOpts) InterfaceResult {
	var res InterfaceResult

	// Validate
	if (opts.SubnetID == "" && opts.PortID == "") || (opts.SubnetID != "" && opts.PortID != "") {
		res.Err = errInvalidInterfaceOpts
	}

	type request struct {
		SubnetID string `json:"subnet_id,omitempty"`
		PortID   string `json:"port_id,omitempty"`
	}

	body := request{SubnetID: opts.SubnetID, PortID: opts.PortID}

	_, err := perigee.Request("PUT", addInterfaceURL(c, id), perigee.Options{
		MoreHeaders: c.Provider.AuthenticatedHeaders(),
		ReqBody:     &body,
		Results:     &res.Resp,
		OkCodes:     []int{200},
	})
	res.Err = err
	return res
}

// RemoveInterface removes an internal router interface, which detaches a
// subnet from the router. You must specify either a SubnetID or PortID, since
// these values are used to identify the router interface to remove.
//
// Unlike AddInterface, you can also specify both a SubnetID and PortID. If you
// choose to specify both, the subnet ID must correspond to the subnet ID of
// the first IP address on the port specified by the port ID. Otherwise, the
// operation will fail and return a 409 Conflict error.
//
// If the router, subnet or port which are referenced do not exist or are not
// visible to you, the operation will fail and a 404 Not Found error will be
// returned. After this operation completes, the port connecting the router
// with the subnet is removed from the subnet for the network.
func RemoveInterface(c *gophercloud.ServiceClient, id string, opts InterfaceOpts) InterfaceResult {
	var res InterfaceResult

	type request struct {
		SubnetID string `json:"subnet_id,omitempty"`
		PortID   string `json:"port_id,omitempty"`
	}

	body := request{SubnetID: opts.SubnetID, PortID: opts.PortID}

	_, err := perigee.Request("PUT", removeInterfaceURL(c, id), perigee.Options{
		MoreHeaders: c.Provider.AuthenticatedHeaders(),
		ReqBody:     &body,
		Results:     &res.Resp,
		OkCodes:     []int{200},
	})
	res.Err = err
	return res
}
