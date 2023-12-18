package main

import (
	"errors"
	"log"
	"net"
	"net/http"
	"regexp"

	"github.com/devilcove/plexus"
	"github.com/devilcove/plexus/database"
	"github.com/gin-contrib/sessions"
	"github.com/gin-gonic/gin"
	"github.com/kr/pretty"
)

func displayAddNetwork(c *gin.Context) {
	session := sessions.Default(c)
	page := getPage(session.Get("user"))
	page.Page = "addNetwork"
	c.HTML(http.StatusOK, "addNetwork", page)

}

func addNetwork(c *gin.Context) {
	var errs error
	network := plexus.Network{}
	if err := c.Bind(&network); err != nil {
		processError(c, http.StatusBadRequest, "invalid network data")
		return
	}
	pretty.Println("addNework: ", network)
	_, cidr, err := net.ParseCIDR(network.AddressString)
	if err != nil {
		log.Println("net.ParseCIDR", network.AddressString)
		processError(c, http.StatusBadRequest, err.Error())
		return
	}
	network.Address = *cidr
	network.AddressString = network.Address.String()
	if !validateNetworkName(network.Name) {
		errs = errors.Join(errs, errors.New("invalid network name"))
	}
	if !validateNetworkAddress(network.Address.IP) {
		errs = errors.Join(errs, errors.New("network address is not private"))
	}
	if errs != nil {
		processError(c, http.StatusBadRequest, errs.Error())
		return
	}
	networks, err := database.GetAllNetworks()
	if err != nil {
		processError(c, http.StatusInternalServerError, "database error "+err.Error())
		return
	}
	for _, net := range networks {
		if net.Name == network.Name {
			processError(c, http.StatusBadRequest, "network name exists")
			return
		}
		if net.Address.IP.Equal(network.Address.IP) {
			processError(c, http.StatusBadRequest, "network CIDR in use by "+net.Name)
			return
		}
	}
	log.Println("network validation complete ... saving network ", network)
	if err := database.SaveNetwork(&network); err != nil {
		processError(c, http.StatusInternalServerError, "unable to save network "+err.Error())
		return
	}
	displayNetworks(c)
}

func displayNetworks(c *gin.Context) {
	networks, err := database.GetAllNetworks()
	if err != nil {
		processError(c, http.StatusInternalServerError, err.Error())
		return
	}
	c.HTML(http.StatusOK, "networks", networks)
}

func deleteNetwork(c *gin.Context) {
	network := c.Param("id")
	if err := database.DeleteNetwork(network); err != nil {
		if errors.Is(err, database.ErrNoResults) {
			processError(c, http.StatusBadRequest, "network does not exist")
			return
		}
		processError(c, http.StatusInternalServerError, "delete network "+err.Error())
		return
	}
	displayNetworks(c)
}

func validateNetworkName(name string) bool {
	if len(name) > 255 {
		return false
	}
	valid := regexp.MustCompile(`^[a-z,-]+$`)
	return valid.MatchString(name)
}

func validateNetworkAddress(address net.IP) bool {
	return address.IsPrivate()

}
