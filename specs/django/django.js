// Specs for Django web service
function Django(cfg, mongo) {
  if (typeof cfg.nWorker !== 'number') {
    throw new Error('`nWorker` is required');
  }
  if (typeof cfg.image !== 'string') {
    throw new Error('`image` is required');
  }
  this._port = cfg.port || 80;
  var env = cfg.env || {};
  env.MONGO_URI = mongo.uri("django-example")

  var containers = new Container(cfg.image).withEnv(env).replicate(cfg.nWorker);
  this._app = new Service("app", containers);

  mongo.connect(mongo.port(), this);
  this.connect(mongo.port(), mongo);
};

Django.prototype.deploy = function(deployment) {
  deployment.deploy(this.services());
};

Django.prototype.services = function() {
  return [this._app];
};

Django.prototype.port = function() {
  return this._port;
};

Django.prototype.connect = function(port, to) {
  var self = this;
  to.services().forEach(function(service) {
    self._app.connect(port, service);
  });
};

module.exports = Django;
