import React, { Component } from 'react';
import * as ReactRedux from 'react-redux';
import PropTypes from 'prop-types';
import autobind from 'autobind-decorator';

import Space from 'containers/Space';
import SpaceExplorer from 'containers/SpaceExplorer';
import { refreshSpaces, refreshCurrentSpace } from 'actions/spaces';

import 'views/SpacesView.scss';

const POLLING_INTERVAL = 10000;

class SpacesView extends Component {
  constructor(props) {
    super(props);
    
    this.props.dispatch(refreshSpaces());
    this.props.dispatch(refreshCurrentSpace());
    this.poll = setInterval(this.handlePollSpaces, POLLING_INTERVAL);
  }

  componentWillUnmount() {
    clearInterval(this.poll);
  }

  @autobind
  handlePollSpaces() {
    this.props.dispatch(refreshSpaces());
    this.props.dispatch(refreshCurrentSpace());
  }

  render() {
    return (
      <div className="SpacesView">
        {this.props.isLoaded && 
          <>
            <SpaceExplorer />
            <Space />
          </>
        }

        {/* TODO: Loader. */}
        {!this.props.isLoaded && 
          <></>
        }
      </div>
    );
  }
}

SpacesView.propTypes = {
  dispatch: PropTypes.func,
  isLoaded: PropTypes.bool.isRequired
};

export default ReactRedux.connect(state => {
  return {
    isLoaded: state.spaces.isLoaded
  };
})(SpacesView);
