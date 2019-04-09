import React, { Component } from 'react';
import * as ReactRedux from 'react-redux';
import PropTypes from 'prop-types';

import Icon from '../components/Icon';

import 'containers/MainContainer.scss';

class MainContainer extends Component {
  constructor(props) {
    super(props);
  }

  render() {
    const { environments } = this.props;
    return (
      <div className="MainContainer layout vertical">
        <div className="Header layout horizontal center">
          space
        </div>
        <div className="EnvironmentList layout vertical">
          {Object.keys(environments).map(id => 
            <div key={id}className="EnvironmentItem layout horizontal center">
              <Icon className="Icon" icon="mirror" size="20"/>
              <div className="Name">{environments[id].name}</div>
              <div className="Endpoint">
                <a>{environments[id].endpoint}</a>
              </div>
            </div>
          )}
        </div>
      </div>
    );  
  }
}

MainContainer.defaultProps = {
};

MainContainer.propTypes = {
  dispatch: PropTypes.func,
  environments: PropTypes.object.isRequired
};

export default ReactRedux.connect(state => {
  return {
    environments: state.environments.byId || {}
  };
})(MainContainer);