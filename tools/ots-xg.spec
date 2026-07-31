%global _build_id_links none
%define __osver %(cat /etc/distrib)
%define __osid %(cat /etc/distrib | sed 's|[a-zA-Z]||g')
%define debug_package %{nil}
%define __channel xg
%define with_tests 0
%define with_docs 0

# Build Options:
#   - Default (Linux RPMs & Tarball):  rpmbuild -ba SPECS/ots-xg.spec
#   - With Windows Release ZIP:        rpmbuild -ba SPECS/ots-xg.spec --with windows
%bcond_with windows

%global goiroot github.com/Luzifer
%global goipath  %{goiroot}/%{srcname}

%global pkgver  1.21.8
%global pkgtag  8440dac
%global srcname ots
%global srcver  master

%define ots_user    ots
%define ots_group   %{ots_user}
%define ots_confdir %{_sysconfdir}/%{name}
%define ots_home    %{_localstatedir}/lib/%{name}
%define ots_logdir  %{_localstatedir}/log/%{name}
%define dist_os     linux_amd64
%define dist_path   /opt/done

Name:           %{srcname}
Version:        %{pkgver}
Release:        10.%{__osver}.%{__channel}
Summary:        One-time-secret sharing platform.
Group:          System Environment/Libraries
License:        ASL 2.0
URL:            https://%{goipath}
Source0:        ots-build.zip
Source1:        ots_builder.sh
Source2:        ots-config.yaml
Source3:        ots.service
Source4:        ots.sysconfig
Patch1:         ots_i18n.yaml.patch
Patch2:         ots_nodejs.patch
BuildRoot:      %{_tmppath}/%{name}-%{version}-%{release}-root-%(%{__id_u} -n)
BuildRequires:  golang >= 1.20, docker-ce, docker-ce-cli, curl, git, tar
%if %{with windows}
BuildRequires:  zip
%endif
Requires:       redis

%description
ots is a one-time-secret sharing platform. The secret is encrypted with a
symmetric 256bit AES encryption in the browser before being sent to the server.
Afterwards an URL containing the ID of the secret and the password is generated.
The password is never sent to the server so the server will never be able
to decrypt the secrets it delivers with a reasonable effort. Also the secret
is immediately deleted on the first read.


%prep
%setup -q -n %{srcname}-%{version}


%build
bash %{SOURCE1} %{pkgver}-%{pkgtag} --no-package %{?_with_windows:--windows}


%install
[ "%{buildroot}" != "/" ] && [ -d "%{buildroot}" ] && rm -rf "%{buildroot}"

mkdir -p %{buildroot}{%{_bindir},%{_sbindir},%{_unitdir},%{ots_confdir}/custom,%{ots_logdir},%{_sysconfdir}/sysconfig,%{ots_home}}

pushd %{name} &>/dev/null
cp -af bin/%{name}-cli_%{dist_os} %{buildroot}%{_bindir}/%{name}-cli
cp -af bin/%{name}_%{dist_os} %{buildroot}%{_sbindir}/%{name}

cp -af %{SOURCE3} %{buildroot}%{_unitdir}/
cp -af %{SOURCE2} %{buildroot}%{ots_confdir}/
cp -af %{SOURCE4} %{buildroot}%{_sysconfdir}/sysconfig/%{name}

mkdir -p distrib/{etc/custom,log,systemd}
pushd distrib &>/dev/null
cp -af %{buildroot}%{_bindir}/%{name}-cli .
cp -af %{buildroot}%{_sbindir}/%{name} .
cp -af %{SOURCE3} systemd/
cp -af %{SOURCE2} etc/
cp -af %{SOURCE4} etc/ots.env
echo "%{name}-%{version}-%{pkgtag}-%{dist_os}" > etc/ots.version
tar Jcf %{dist_path}/%{name}-%{version}-%{pkgtag}%{dist}.%{__channel}.%{_arch}.tar.xz *
popd &>/dev/null

%if %{with windows}
mkdir -p distrib_win/{bin,etc/custom,log}
pushd distrib_win &>/dev/null
cp -af ../bin/ots_windows_amd64.exe bin/ots.exe
cp -af ../bin/ots-cli_windows_amd64.exe bin/ots-cli.exe
cp -af %{SOURCE2} etc/ots-config.yaml
cp -af %{SOURCE4} etc/ots.env
sed -i 's|/etc/ots/ots-config.yaml|c:/inetd/ots/etc/ots-config.yaml|g' etc/ots.env
sed -i 's|/etc/ots/custom|c:/inetd/ots/etc/custom|g' etc/ots-config.yaml
echo "%{name}-%{version}-%{pkgtag}-windows_amd64" > etc/ots.version
zip -9r %{dist_path}/%{name}-%{version}-%{pkgtag}.win.%{__channel}.%{_arch}.zip etc bin/*.exe log
popd &>/dev/null
rm -rf distrib_win
%endif

popd &>/dev/null


%pre
getent group %{ots_group} >/dev/null || groupadd -r %{ots_group}
getent passwd %{ots_user} >/dev/null || \
    useradd -r -g %{ots_user} -d %{ots_home} -s /sbin/nologin \
    -c "OTS Server" %{ots_user}
exit 0

%post
%systemd_post %{name}.service

%preun
%systemd_preun %{name}.service

%postun
%systemd_postun_with_restart %{name}.service


%clean
[ "%{buildroot}" != "/" ] && [ -d "%{buildroot}" ] && rm -rf "%{buildroot}"


%files
%defattr(-,root,root,-)
%{!?_licensedir:%global license %%doc}
%license %{name}/LICENSE
%doc %{name}/Dockerfile* %{name}/README.md %{name}/SECURITY.md
%{_bindir}/%{name}-cli
%{_sbindir}/%{name}
%{_unitdir}/%{name}.service
%config(noreplace) %{ots_confdir}
%config(noreplace) %{_sysconfdir}/sysconfig/%{name}
%attr(-,%{ots_user},%{ots_group}) %dir %{ots_home}
%attr(-,%{ots_user},%{ots_group}) %dir %{ots_logdir}


%changelog
* Sat Jul 04 2026 Mis Center <mis@criticalsys.net> - 1.21.8-1
- Latest upstream release
