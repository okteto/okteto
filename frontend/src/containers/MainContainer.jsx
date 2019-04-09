import React, { Component } from 'react';
import * as ReactRedux from 'react-redux';
import PropTypes from 'prop-types';

import 'containers/MainContainer.scss';

class MainContainer extends Component {
  constructor(props) {
    super(props);
  }

  render() {
    return (
      <div className="MainContainer">
        Welcome to Okteto!
      </div>
    );
  }
}

MainContainer.propTypes = {
  dispatch: PropTypes.func
};

export default ReactRedux.connect(() => {})(MainContainer);