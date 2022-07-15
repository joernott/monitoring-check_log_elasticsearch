#
# spec file for package monitoring-check_log_elasticsearch
#
Name:           monitoring-check_log_elasticsearch
Version:        %{version}
Release:        %{release}
Summary:        Icinga2/Nagios check for log files stored in Elasticsearch
License:        BSD
Group:          Sytem/Utilities
Vendor:         Ott-Consult UG
Packager:       Joern Ott
Url:            https://github.com/joernott/monitoring-check_f5_throughput
Source0:        monitoring-check_log_elasticsearch-%{version}.tar.gz
BuildArch:      x86_64

%description
A check for Icinga2 or Nagios to check logs stored in Elasticsearch

%prep
cd "$RPM_BUILD_DIR"
rm -rf *
tar -xzf "%{SOURCE0}"
STATUS=$?
if [ $STATUS -ne 0 ]; then
  exit $STATUS
fi
/usr/bin/chmod -Rf a+rX,u+w,g-w,o-w .

%build
cd "$RPM_BUILD_DIR/monitoring-check_log_elasticsearch-%{version}/check_log_elasticsearch"
go get -u -v
go build -v

%install
install -Dpm 0755 %{name}-%{version}/check_log_elasticsearch/check_log_elasticsearch "%{buildroot}/usr/lib64/nagios/plugins/check_log_elasticsearch"

%files
%defattr(-,root,root,755)
%attr(755, root, root) /usr/lib64/nagios/plugins/check_log_elasticsearch

%changelog
* Fri Jul 15 2022 Joern Ott <joern.ott@ott-consult.de>
- Initial version