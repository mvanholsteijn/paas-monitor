resource "google_compute_region_instance_group_manager" "paas-monitor" {
  name               = "paas-monitor"
  base_instance_name = "paas-monitor"

  version {
    instance_template = google_compute_instance_template.paas-monitor.id
  }

  target_size = 1
  update_policy {
    type                           = "PROACTIVE"
    minimal_action                 = "RESTART"
    most_disruptive_allowed_action = "REPLACE"
    max_surge_fixed                = number_of_zones + 1
    max_unavailable_fixed          = number_of_zones
  }

  named_port {
    name = "paas-monitor"
    port = 80
  }

  auto_healing_policies {
    health_check      = google_compute_health_check.paas-monitor.id
    initial_delay_sec = 30
  }
}

resource "google_compute_health_check" "paas-monitor" {
  name        = "paas-monitor"
  description = "paas-monitor health check"

  timeout_sec         = 1
  check_interval_sec  = 1
  healthy_threshold   = 4
  unhealthy_threshold = 5

  http_health_check {
    port_name    = "paas-monitor"
    request_path = "/health"
    response     = "ok"
  }
}


data "google_compute_zones" "available" {
}

locals {
  number_of_zones = length(data.google_compute_zones.available.names)
}

resource "google_compute_instance_template" "paas-monitor" {
  name_prefix = "paas-monitor"
  description = "showing what happens"

  instance_description = "paas-monitor"
  machine_type         = "e2-micro"

  disk {
    source_image = "cos-cloud/cos-stable"
    auto_delete  = true
    boot         = true
  }

  metadata = {
    "user-data" = file("user-data.yaml")
  }

  tags = ["http-server"]

  network_interface {
    network = "default"

    access_config {}
  }

  scheduling {
    automatic_restart   = false
    preemptible         = true
    on_host_maintenance = "TERMINATE"
  }

  lifecycle {
    create_before_destroy = true
  }
}
