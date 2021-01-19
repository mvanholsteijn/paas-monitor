function MonitorController($scope, $interval, $http) {
    var monitor;
    $scope.error_count = 0;
    $scope.responses = [];
    $scope.stats = {};
    $scope.last_status = "";

    $scope.startMonitor =  function() {
      // Don't start a new monitor if one is already active
      if ( angular.isDefined(monitor) ) return;

      monitor = $interval(function() {
            $scope.callService();
        }, 250);
      };

    $scope.stopMonitor = function() {
      if ( angular.isDefined(monitor) )  {
	$interval.cancel(monitor);
	monitor = undefined;
      }
    }

    $scope.$on('$destroy', function() {
      // Make sure that the monitor is destroyed too
      $scope.stopMonitor();
    });


    function baseURL() {
       return document.location.href.replace(/[^/]*$/, "").replace(/\/$/, "")
    }

    $scope.callService = function() {
            var startTime = new Date().getTime();

	    $http.get(baseURL() + '/status', {headers: {Connection: 'close'}})
		.success(function(response) {
		    var responseTime = new Date().getTime() - startTime;
		    var key = response.key;
		    $scope.msg = key;
		    if($scope.stats.hasOwnProperty(key)) {
			    $scope.stats[key].count += 1;
			    $scope.stats[key].servercount = response.servercount;
			    $scope.stats[key].total += responseTime;
			    $scope.stats[key].last = responseTime;
			    $scope.stats[key].avg = Math.round($scope.stats[key].total / $scope.stats[key].count);
		    } else {
			    $scope.stats[key] = { count : 1, servercount: response.servercount, last : responseTime, total : responseTime, avg : responseTime };
			    $scope.responses.push(response) ;
		    }

		    var current = _.find($scope.responses, function(item) {return item.key === response.key});

		    current.message = response.message;
		    current.release = response.release;
		    current.servercount = response.servercount;
		    current.cpu = response.cpu;
		    current.healthy = response.healthy;
		}).
		error(function(data, status, headers, config) {
			console.log(data);
			console.log(status);
			console.log(headers);
			console.log(config);
			$scope.error_count++;
			$scope.last_status = "" + status + ", " + data;
			if($scope.error_count % 250 == 0) {
				$scope.stopMonitor();
				$scope.last_status = ": more than 250 errors, stopped monitoring";
			}
		});
   }

   $scope.stopService = function() {
	    $http.get(baseURL() + '/stop')
		.success(function(response) {
			console.log('a service was stopped');
		}).
		error(function(data, status, headers, config) {
			console.log(data);
			console.log(status);
			console.log(headers);
			console.log(config);
		});
   }

   $scope.increaseCpu = function() {
	    $http.get(baseURL() + '/increase-cpu')
		.success(function(response) {
			console.log('/increase-cpu was called');
		}).
		error(function(data, status, headers, config) {
			console.log(data);
			console.log(status);
			console.log(headers);
			console.log(config);
		});
   }

   $scope.decreaseCpu = function() {
	    $http.get(baseURL() + '/decrease-cpu')
		.success(function(response) {
			console.log('/decrease-cpu was called');
		}).
		error(function(data, status, headers, config) {
			console.log(data);
			console.log(status);
			console.log(headers);
			console.log(config);
		});
   }

   $scope.toggleHealth = function() {
	    $http.get(baseURL() + '/toggle-health')
		.success(function(response) {
			console.log('a service was made unhealthy');
		}).
		error(function(data, status, headers, config) {
			console.log(data);
			console.log(status);
			console.log(headers);
			console.log(config);
		});
   }

   $scope.startMonitor();
}
