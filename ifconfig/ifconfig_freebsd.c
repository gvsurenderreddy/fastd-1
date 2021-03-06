#include <errno.h>
#include <string.h>
#include <net/if.h>
#include <net/if_var.h>
#include <sys/sockio.h>
#include <netinet/in.h>
#include <netinet/in_var.h>
#include <netinet6/in6_var.h>
#include <netinet6/nd6.h>
#include "ifconfig.h"

static int ioctl_fd4 = 0;
static int ioctl_fd6 = 0;

int
set_fd(sa_family_t af){
	errno = 0;
	switch(af){
	case AF_INET:
		ioctl_fd4 = socket(af, SOCK_DGRAM, 0);
		return errno;
	case AF_INET6:
		ioctl_fd6 = socket(af, SOCK_DGRAM, 0);
		return errno;
	default:
		return EAFNOSUPPORT;
	}
}

static inline void
mask32(struct sockaddr_in *sa){
	sa->sin_len    = sizeof(struct sockaddr_in);
	sa->sin_family = AF_INET;
	memset(&sa->sin_addr, '\xff', sizeof(struct in_addr));
}

// Sets a variable prefixlen
static inline void
mask128(struct sockaddr_in6 *sa, uint8_t len){
	u_char *cp;

	sa->sin6_len    = sizeof(struct sockaddr_in6);
	sa->sin6_family = AF_INET6;

	memset((void *)&sa->sin6_addr, 0x00, sizeof(sa->sin6_addr));
	for (cp = (u_char *)&sa->sin6_addr; len > 7; len -= 8)
		*cp++ = 0xff;
	*cp = 0xff << (8 - len);
}

int
if_clone(char* ifname, void* data)
{
	int result;
	struct ifreq ifr;

	bzero(&ifr, sizeof(ifr));
	strlcpy(ifr.ifr_name, ifname, sizeof(ifr.ifr_name));

	if (data) {
		ifr.ifr_data = data;
		result = ioctl(ioctl_fd4, SIOCIFCREATE2, &ifr);
	} else {
		result = ioctl(ioctl_fd4, SIOCIFCREATE, &ifr);
	}

	strlcpy(ifname, ifr.ifr_name, sizeof(ifr.ifr_name));

	return result;
}

int
if_destroy(char* ifname)
{
	struct ifreq ifr;

	bzero(&ifr, sizeof(ifr));
	strlcpy(ifr.ifr_name, ifname, sizeof(ifr.ifr_name));

	return ioctl(ioctl_fd4, SIOCIFDESTROY, &ifr);
}

int
get_mtu(char *ifname, int *mtu){
	int error;
	struct ifreq ifr;

	bzero(&ifr, sizeof(ifr));
	strlcpy(ifr.ifr_name, ifname, sizeof(ifr.ifr_name));

	error = ioctl(ioctl_fd4, SIOCGIFMTU, &ifr);

	if (!error)
		*mtu = ifr.ifr_mtu;

	return error;
}

int
set_mtu(char *ifname, int mtu){
	struct ifreq ifr;

	bzero(&ifr, sizeof(ifr));
	strlcpy(ifr.ifr_name, ifname, sizeof(ifr.ifr_name));
	ifr.ifr_mtu = mtu;

	return ioctl(ioctl_fd4, SIOCSIFMTU, &ifr);
}

int
get_descr(char* ifname, char* descr, size_t descrlen){
	struct ifreq ifr;

	bzero(&ifr, sizeof(ifr));
	strlcpy(ifr.ifr_name, ifname, sizeof(ifr.ifr_name));

	ifr.ifr_buffer.buffer = descr;
	ifr.ifr_buffer.length = descrlen;

	return ioctl(ioctl_fd4, SIOCGIFDESCR, &ifr);
}

int
set_descr(char* ifname, char* descr){
	struct ifreq ifr;
	bzero(&ifr, sizeof(ifr));
	strlcpy(ifr.ifr_name, ifname, sizeof(ifr.ifr_name));

	ifr.ifr_buffer.buffer = descr;
	ifr.ifr_buffer.length = strlen(descr) + 1;

	if (ifr.ifr_buffer.length == 1) {
		ifr.ifr_buffer.buffer = NULL;
		ifr.ifr_buffer.length = 0;
	}

	return ioctl(ioctl_fd4, SIOCSIFDESCR, &ifr);
}

int
remove_addr4(char* ifname)
{
	struct ifreq ifr;
	bzero(&ifr, sizeof(ifr));
	strlcpy(ifr.ifr_name, ifname, sizeof(ifr.ifr_name));

	return ioctl(ioctl_fd4, SIOCDIFADDR, &ifr);
}

int
remove_addr6(char* ifname, struct sockaddr_in6 *addr)
{
	struct in6_ifreq req;
	bzero(&req, sizeof(req));
	strlcpy(req.ifr_name, ifname, sizeof(req.ifr_name));
	memcpy(&req.ifr_addr, addr, sizeof(struct sockaddr_in6));

	return ioctl(ioctl_fd6, SIOCDIFADDR_IN6, &req);
}
int
set_addr4(char* ifname, struct sockaddr_in *addr, struct sockaddr_in *netmask)
{
	struct ifaliasreq req;
	bzero(&req, sizeof(req));

	strlcpy(req.ifra_name, ifname,  sizeof(req.ifra_name));
	memcpy(&req.ifra_addr, addr,    sizeof(struct sockaddr_in));
	memcpy(&req.ifra_mask, netmask, sizeof(struct sockaddr_in));

	return ioctl(ioctl_fd4, SIOCAIFADDR, &req);
}

int
add_addr4_ptp(char* ifname, struct sockaddr_in *addr, struct sockaddr_in *dstaddr)
{
	struct ifaliasreq req;
	bzero(&req, sizeof(req));

	strlcpy(req.ifra_name,      ifname,  sizeof(req.ifra_name));
	memcpy(&req.ifra_addr,      addr,    sizeof(struct sockaddr_in));
	memcpy(&req.ifra_broadaddr, dstaddr, sizeof(struct sockaddr_in));
	mask32((struct sockaddr_in *)&req.ifra_mask);

	return ioctl(ioctl_fd4, SIOCAIFADDR, &req);
}

int
add_addr6_ptp(char* ifname, struct sockaddr_in6 *addr, struct sockaddr_in6 *dstaddr)
{
	struct in6_aliasreq req;
	bzero(&req, sizeof(req));

	strlcpy(req.ifra_name,    ifname,  sizeof(req.ifra_name));
	memcpy(&req.ifra_addr,    addr,    sizeof(struct sockaddr_in6));
	memcpy(&req.ifra_dstaddr, dstaddr, sizeof(struct sockaddr_in6));
	mask128(&req.ifra_prefixmask, 128);

	req.ifra_lifetime.ia6t_vltime = ND6_INFINITE_LIFETIME;
	req.ifra_lifetime.ia6t_pltime = ND6_INFINITE_LIFETIME;

	return ioctl(ioctl_fd6, SIOCAIFADDR_IN6, &req);
}

// Add a /64 IPv6 address
int
add_addr6(char* ifname, struct sockaddr_in6 *addr, uint8_t prefixlen)
{
	struct in6_aliasreq req;
	bzero(&req, sizeof(req));

	strlcpy(req.ifra_name,    ifname,  sizeof(req.ifra_name));
	memcpy(&req.ifra_addr,    addr,    sizeof(struct sockaddr_in6));
	mask128(&req.ifra_prefixmask, prefixlen);

	req.ifra_lifetime.ia6t_vltime = ND6_INFINITE_LIFETIME;
	req.ifra_lifetime.ia6t_pltime = ND6_INFINITE_LIFETIME;

	return ioctl(ioctl_fd6, SIOCAIFADDR_IN6, &req);
}

int
get_drv_spec(char* ifname, unsigned long cmd, void *data, size_t len)
{
	struct ifdrv ifd;
	bzero(&ifd, sizeof(ifd));

	ifd.ifd_cmd  = cmd;
	ifd.ifd_data = data;
	ifd.ifd_len  = len;
	strlcpy(ifd.ifd_name, ifname, sizeof(ifd.ifd_name));

	return ioctl(ioctl_fd4, SIOCGDRVSPEC, &ifd);
}

int
set_drv_spec(char* ifname, unsigned long cmd, void *data, size_t len)
{
	struct ifdrv ifd;
	bzero(&ifd, sizeof(ifd));

	ifd.ifd_cmd  = cmd;
	ifd.ifd_data = data;
	ifd.ifd_len  = len;
	strlcpy(ifd.ifd_name, ifname, sizeof(ifd.ifd_name));

	return ioctl(ioctl_fd4, SIOCSDRVSPEC, &ifd);
}
