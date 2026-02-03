Name:           librato
Version:        %{_version}
Release:        1%{?dist}
Summary:        Automatic music library organizer daemon

License:        MIT
URL:            https://github.com/pkazmierczak/librato
Source0:        librato
Source1:        librato.service
Source2:        config.daemon.json

%description
Librato is a tool that automatically organizes your music library
based on ID3 tags. It can run as a CLI tool or as a background daemon
that watches a directory for new music files and moves them to your
library using configurable patterns.

Features include:
- Organize music by artist, album, genre, and other ID3 tags
- Watch directory mode for automatic file processing
- Quarantine untagged files for manual review
- Systemd service integration
- Dry-run mode for previewing changes

%install
# Install binary
install -D -m 0755 %{SOURCE0} %{buildroot}%{_bindir}/librato

# Install systemd service
install -D -m 0644 %{SOURCE1} %{buildroot}%{_unitdir}/librato.service

# Install example config
install -D -m 0644 %{SOURCE2} %{buildroot}%{_sysconfdir}/librato/config.json.example

# Create state directories
install -d -m 0755 %{buildroot}%{_sharedstatedir}/librato
install -d -m 0755 %{buildroot}%{_rundir}/librato

%pre
# Create system user if it doesn't exist
getent group librato >/dev/null || groupadd -r librato
getent passwd librato >/dev/null || useradd -r -g librato -d %{_sharedstatedir}/librato -s /sbin/nologin -c "Librato daemon" librato

%post
%systemd_post librato.service

# Copy example config if no config exists
if [ ! -f %{_sysconfdir}/librato/config.json ]; then
    if [ -f %{_sysconfdir}/librato/config.json.example ]; then
        cp %{_sysconfdir}/librato/config.json.example %{_sysconfdir}/librato/config.json
        chown librato:librato %{_sysconfdir}/librato/config.json
    fi
fi

# Set ownership of state directories
chown librato:librato %{_sharedstatedir}/librato
chown librato:librato %{_rundir}/librato

%preun
%systemd_preun librato.service

%postun
%systemd_postun_with_restart librato.service

%files
%{_bindir}/librato
%{_unitdir}/librato.service
%dir %{_sysconfdir}/librato
%config(noreplace) %{_sysconfdir}/librato/config.json.example
%dir %attr(0755, librato, librato) %{_sharedstatedir}/librato
%dir %attr(0755, librato, librato) %{_rundir}/librato
